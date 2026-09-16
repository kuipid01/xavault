package reconciliation

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/alerts"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

type Service struct {
	db          *gorm.DB
	provider    paymentproviders.PaymentProvider
	wallets     *wallets.WalletService
	withdrawals *withdrawals.WithdrawalService
	alerts      *alerts.Service
}

// depositCheckoutTTL matches the Paystack checkout link TTL the mobile app
// enforces (CHECKOUT_LINK_TTL_MS = 24h). Pending deposits older than this are
// expired instead of being polled against the provider forever.
const depositCheckoutTTL = 24 * time.Hour

// withdrawalResolutionTTL bounds how long the worker keeps asking the provider
// about a pending/processing withdrawal. Paystack transfers normally resolve in
// minutes; anything still open after 48h is marked failed so polling stops.
const withdrawalResolutionTTL = 48 * time.Hour

func (s *Service) SetAlertPublisher(publisher *alerts.Service) { s.alerts = publisher }

func (s *Service) publishAlert(ctx context.Context, input alerts.Input) {
	if s.alerts == nil {
		return
	}
	if err := s.alerts.Publish(ctx, input); err != nil {
		log.Printf(`{"level":"error","event":"operational_alert_enqueue_failed","alert_type":%q,"error":%q}`, input.EventType, err.Error())
	}
}

type RunResult struct {
	RunID            string    `json:"run_id"`
	CheckedAt        time.Time `json:"checked_at"`
	Findings         int       `json:"findings"`
	ProcessingHalted bool      `json:"processing_halted"`
}

type ReferenceInspection struct {
	Reference  string                   `json:"reference"`
	Deposit    *wallets.WalletDeposit   `json:"deposit,omitempty"`
	Withdrawal *withdrawals.Withdrawal  `json:"withdrawal,omitempty"`
	Events     []map[string]interface{} `json:"events"`
}

func NewService(
	db *gorm.DB,
	provider paymentproviders.PaymentProvider,
	walletService *wallets.WalletService,
	withdrawalService *withdrawals.WithdrawalService,
) *Service {
	return &Service{db: db, provider: provider, wallets: walletService, withdrawals: withdrawalService}
}

// RunFinancialCheck validates invariants that must always hold before money is
// allowed to move. Critical findings automatically engage the processing halt.
func (s *Service) RunFinancialCheck(ctx context.Context) (*RunResult, error) {
	runID := uuid.NewString()
	now := time.Now()
	var findings []Finding

	var walletRows []wallets.Wallet
	if err := s.db.WithContext(ctx).Find(&walletRows).Error; err != nil {
		return nil, err
	}
	for _, wallet := range walletRows {
		if wallet.AvailableBalance < 0 || wallet.LockedBalance < 0 || wallet.LedgerBalance < 0 {
			findings = append(findings, Finding{
				RunID: runID, Severity: SeverityCritical, Code: "negative_wallet_balance",
				EntityType: "wallet", EntityID: wallet.ID,
				Expected: "all balances >= 0",
				Actual:   fmt.Sprintf("available=%d locked=%d ledger=%d", wallet.AvailableBalance, wallet.LockedBalance, wallet.LedgerBalance),
			})
		}
		if wallet.AvailableBalance+wallet.LockedBalance != wallet.LedgerBalance {
			findings = append(findings, Finding{
				RunID: runID, Severity: SeverityCritical, Code: "wallet_balance_invariant_failed",
				EntityType: "wallet", EntityID: wallet.ID,
				Expected: fmt.Sprintf("%d", wallet.LedgerBalance),
				Actual:   fmt.Sprintf("%d", wallet.AvailableBalance+wallet.LockedBalance),
			})
		}
	}

	accountingFindings, err := s.accountingFindings(ctx, runID)
	if err != nil {
		return nil, err
	}
	findings = append(findings, accountingFindings...)
	if len(findings) > 0 {
		if err := s.db.WithContext(ctx).Create(&findings).Error; err != nil {
			return nil, err
		}
	}

	halt := false
	for _, finding := range findings {
		if finding.Severity == SeverityCritical {
			halt = true
			break
		}
	}
	if halt {
		reason := fmt.Sprintf("financial reconciliation %s found %d invariant violation(s)", runID, len(findings))
		if err := s.halt(ctx, reason); err != nil {
			return nil, err
		}
		log.Printf(`{"level":"critical","event":"financial_processing_halted","run_id":%q,"findings":%d}`, runID, len(findings))
		s.publishAlert(ctx, alerts.Input{Severity: alerts.SeverityCritical, EventType: "financial_processing_halted", Title: "Financial processing halted", Message: reason, Fingerprint: "financial-processing-halted", Metadata: map[string]interface{}{"run_id": runID, "findings": len(findings)}})
	}

	return &RunResult{RunID: runID, CheckedAt: now, Findings: len(findings), ProcessingHalted: halt}, nil
}

func (s *Service) accountingFindings(ctx context.Context, runID string) ([]Finding, error) {
	var findings []Finding
	type duplicate struct {
		Reference string
		Count     int64
	}
	var ledgerDuplicates []duplicate
	if err := s.db.WithContext(ctx).Model(&wallets.WalletLedger{}).
		Select("reference, COUNT(*) AS count").
		Group("reference").Having("COUNT(*) > 1").Scan(&ledgerDuplicates).Error; err != nil {
		return nil, err
	}
	for _, item := range ledgerDuplicates {
		findings = append(findings, Finding{
			RunID: runID, Severity: SeverityCritical, Code: "duplicate_ledger_reference",
			EntityType: "ledger", EntityID: item.Reference, Expected: "1", Actual: fmt.Sprintf("%d", item.Count),
		})
	}

	type walletTotal struct {
		WalletID       string
		StoredLedger   int64
		ComputedLedger int64
		StoredLocked   int64
		ComputedLocked int64
	}
	var totals []walletTotal
	if err := s.db.WithContext(ctx).Raw(`
		SELECT w.id AS wallet_id,
		       w.ledger_balance AS stored_ledger,
		       w.locked_balance AS stored_locked,
		       COALESCE(SUM(CASE
		         WHEN l.type IN ('credit','refund') THEN l.amount
		         WHEN l.type IN ('debit','withdrawal_debit') THEN -l.amount
		         ELSE 0 END), 0) AS computed_ledger,
		       COALESCE(SUM(CASE
		         WHEN l.type = 'lock' THEN l.amount
		         WHEN l.type = 'unlock' THEN -l.amount
		         WHEN l.type = 'withdrawal_debit' THEN -l.amount
		         WHEN l.type = 'refund' AND l.source = 'withdrawal' THEN l.amount
		         WHEN l.type = 'debit' AND l.source = 'plan_cancellation_fee' THEN -l.amount
		         ELSE 0 END), 0) AS computed_locked
		FROM wallets w
		LEFT JOIN wallet_ledgers l ON l.wallet_id = w.id
		GROUP BY w.id, w.ledger_balance, w.locked_balance`).
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	for _, total := range totals {
		if total.StoredLedger != total.ComputedLedger {
			findings = append(findings, Finding{
				RunID: runID, Severity: SeverityCritical, Code: "wallet_ledger_total_mismatch",
				EntityType: "wallet", EntityID: total.WalletID,
				Expected: fmt.Sprintf("%d", total.StoredLedger), Actual: fmt.Sprintf("%d", total.ComputedLedger),
			})
		}
		if total.StoredLocked != total.ComputedLocked {
			findings = append(findings, Finding{
				RunID: runID, Severity: SeverityCritical, Code: "wallet_locked_ledger_mismatch",
				EntityType: "wallet", EntityID: total.WalletID,
				Expected: fmt.Sprintf("%d", total.StoredLocked), Actual: fmt.Sprintf("%d", total.ComputedLocked),
			})
		}
	}

	type moneyMismatch struct {
		ID        string
		Reference string
		Expected  int64
		Actual    int64
		Entries   int64
	}
	// The ledger credit for a successful deposit is the NET amount the wallet
	// received (WalletCreditAmount), not the gross charge (d.amount includes
	// the provider fee: e.g. 100000 requested -> 101523 charged -> 100000
	// credited). Comparing against d.amount flags every fee-charging deposit
	// as a critical mismatch and halts all money movement.
	var depositMismatches []moneyMismatch
	if err := s.db.WithContext(ctx).Raw(`
		SELECT d.id, d.reference, d.wallet_credit_amount AS expected,
		       COALESCE(SUM(l.amount),0) AS actual, COUNT(l.id) AS entries
		FROM wallet_deposits d
		LEFT JOIN wallet_ledgers l
		  ON l.source_id::uuid = d.id AND l.type = 'credit'
		WHERE d.status = 'success'
		GROUP BY d.id, d.reference, d.wallet_credit_amount
		HAVING COUNT(l.id) <> 1 OR COALESCE(SUM(l.amount),0) <> d.wallet_credit_amount`).
		Scan(&depositMismatches).Error; err != nil {
		return nil, err
	}
	for _, item := range depositMismatches {
		findings = append(findings, Finding{
			RunID: runID, Severity: SeverityCritical, Code: "deposit_ledger_mismatch",
			EntityType: "deposit", EntityID: item.ID,
			Expected: fmt.Sprintf("amount=%d entries=1", item.Expected),
			Actual:   fmt.Sprintf("amount=%d entries=%d reference=%s", item.Actual, item.Entries, item.Reference),
		})
	}

	var withdrawalMismatches []moneyMismatch
	if err := s.db.WithContext(ctx).Raw(`
		SELECT wd.id, wd.reference, wd.amount AS expected,
		       COALESCE(SUM(l.amount),0) AS actual, COUNT(l.id) AS entries
		FROM withdrawals wd
		LEFT JOIN wallet_ledgers l
		  ON l.source_id::uuid = wd.id AND l.type = 'withdrawal_debit'
		WHERE wd.status IN ('paid','reversed')
		GROUP BY wd.id, wd.reference, wd.amount
		HAVING COUNT(l.id) <> 1 OR COALESCE(SUM(l.amount),0) <> wd.amount`).
		Scan(&withdrawalMismatches).Error; err != nil {
		return nil, err
	}
	for _, item := range withdrawalMismatches {
		findings = append(findings, Finding{
			RunID: runID, Severity: SeverityCritical, Code: "withdrawal_debit_ledger_mismatch",
			EntityType: "withdrawal", EntityID: item.ID,
			Expected: fmt.Sprintf("amount=%d entries=1", item.Expected),
			Actual:   fmt.Sprintf("amount=%d entries=%d reference=%s", item.Actual, item.Entries, item.Reference),
		})
	}

	var reversalMismatches []moneyMismatch
	if err := s.db.WithContext(ctx).Raw(`
		SELECT wd.id, wd.reference, wd.amount AS expected,
		       COALESCE(SUM(l.amount),0) AS actual, COUNT(l.id) AS entries
		FROM withdrawals wd
		LEFT JOIN wallet_ledgers l
		  ON l.source_id::uuid = wd.id AND l.type = 'refund'
		WHERE wd.status = 'reversed'
		GROUP BY wd.id, wd.reference, wd.amount
		HAVING COUNT(l.id) <> 1 OR COALESCE(SUM(l.amount),0) <> wd.amount`).
		Scan(&reversalMismatches).Error; err != nil {
		return nil, err
	}
	for _, item := range reversalMismatches {
		findings = append(findings, Finding{
			RunID: runID, Severity: SeverityCritical, Code: "withdrawal_reversal_ledger_mismatch",
			EntityType: "withdrawal", EntityID: item.ID,
			Expected: fmt.Sprintf("refund=%d entries=1", item.Expected),
			Actual:   fmt.Sprintf("refund=%d entries=%d reference=%s", item.Actual, item.Entries, item.Reference),
		})
	}
	return findings, nil
}

func (s *Service) halt(ctx context.Context, reason string) error {
	now := time.Now()
	control := Control{ID: 1}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.FirstOrCreate(&control, Control{ID: 1}).Error; err != nil {
			return err
		}
		return tx.Model(&control).Updates(map[string]interface{}{
			"processing_halted": true,
			"reason":            reason,
			"halted_at":         &now,
		}).Error
	})
}

func (s *Service) Resume(ctx context.Context, operatorID, reason string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Control{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"processing_halted": false,
			"reason":            "",
			"halted_at":         nil,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&OperatorAction{
			OperatorID: operatorID, Action: "resume_financial_processing", Details: reason,
		}).Error
	})
}

// ReconcileStuck asks Paystack for conclusive provider state. It never creates
// a second transfer and always reuses the stored provider/local reference.
func (s *Service) ReconcileStuck(ctx context.Context) error {
	cutoff := time.Now().Add(-15 * time.Minute)
	var stuckWithdrawals []withdrawals.Withdrawal
	if err := s.db.WithContext(ctx).
		Where("status IN ? AND created_at < ?", []string{withdrawals.WithdrawalStatusPending, withdrawals.WithdrawalStatusProcessing}, cutoff).
		Order("created_at ASC").Limit(100).Find(&stuckWithdrawals).Error; err != nil {
		return err
	}
	for _, withdrawal := range stuckWithdrawals {
		// Bug fix: a withdrawal that never resolved within the TTL stops
		// being polled against the provider on every run. No money has moved
		// locally (the debit happens at completion), so marking it failed is
		// safe; a late provider success is still honored on webhook.
		if time.Since(withdrawal.CreatedAt) > withdrawalResolutionTTL {
			result := s.withdrawals.FailStaleWithdrawal(withdrawal.Reference, "withdrawal unresolved within the reconciliation window")
			if result.Error != nil {
				log.Printf(`{"level":"error","event":"withdrawal_expire_failed","reference":%q,"error":%q}`, withdrawal.Reference, fmt.Sprint(result.Error))
			} else {
				log.Printf(`{"level":"info","event":"withdrawal_expired","reference":%q}`, withdrawal.Reference)
			}
			continue
		}
		reference := strings.TrimSpace(withdrawal.ProviderReference)
		if reference == "" {
			reference = withdrawal.Reference
		}
		verify, err := s.provider.VerifyWithdrawal(ctx, reference)
		if err != nil {
			log.Printf(`{"level":"error","event":"withdrawal_reconcile_provider_error","reference":%q,"error":%q}`, reference, err.Error())
			s.publishAlert(ctx, alerts.Input{Severity: alerts.SeverityError, EventType: "withdrawal_provider_timeout", Title: "Withdrawal verification failed", Message: "Provider verification failed for a stuck withdrawal", Fingerprint: "withdrawal-provider:" + reference, Metadata: map[string]interface{}{"reference": reference, "error": err.Error()}})
			continue
		}
		data := verify.Data
		switch strings.ToLower(data.Status) {
		case "success":
			result := s.withdrawals.CompleteProviderWithdrawalWithDetails(reference, data.Status, data.Amount, data.Currency, data.TransferCode)
			if result.Error != nil {
				log.Printf(`{"level":"error","event":"withdrawal_reconcile_failed","reference":%q}`, reference)
			}
		case "failed", "reversed":
			result := s.withdrawals.FailProviderWithdrawal(reference, data.Status, data.Reason)
			if result.Error != nil {
				log.Printf(`{"level":"error","event":"withdrawal_reconcile_failed","reference":%q}`, reference)
			}
		}
	}

	var deposits []wallets.WalletDeposit
	if err := s.db.WithContext(ctx).Where("status = ? AND created_at < ?", "pending", cutoff).
		Order("created_at ASC").Limit(100).Find(&deposits).Error; err != nil {
		return err
	}
	for _, deposit := range deposits {
		// Bug fix: an abandoned checkout link never completes. Expire it so
		// this deposit stops being checked against the provider on every
		// reconciliation run - even if the app's dismiss call never landed.
		if time.Since(deposit.CreatedAt) > depositCheckoutTTL {
			if err := s.wallets.ExpireStaleDeposit(deposit.Reference); err != nil {
				log.Printf(`{"level":"error","event":"deposit_expire_failed","reference":%q,"error":%q}`, deposit.Reference, err.Error())
			} else {
				log.Printf(`{"level":"info","event":"deposit_expired","reference":%q}`, deposit.Reference)
			}
			continue
		}
		verify, err := s.provider.VerifyDeposit(ctx, deposit.Reference)
		if err != nil {
			log.Printf(`{"level":"error","event":"deposit_reconcile_provider_error","reference":%q}`, deposit.Reference)
			s.publishAlert(ctx, alerts.Input{Severity: alerts.SeverityError, EventType: "deposit_provider_timeout", Title: "Deposit verification failed", Message: "Provider verification failed for a stuck deposit", Fingerprint: "deposit-provider:" + deposit.Reference, Metadata: map[string]interface{}{"reference": deposit.Reference, "error": err.Error()}})
			continue
		}
		data := verify.Data
		if strings.ToLower(data.Status) != "success" {
			continue
		}
		if data.Reference != deposit.Reference || data.Amount != deposit.Amount || !strings.EqualFold(data.Currency, deposit.Currency) {
			reason := fmt.Sprintf("deposit verification mismatch for %s", deposit.Reference)
			_ = s.halt(ctx, reason)
			log.Printf(`{"level":"critical","event":"deposit_amount_mismatch","reference":%q}`, deposit.Reference)
			s.publishAlert(ctx, alerts.Input{Severity: alerts.SeverityCritical, EventType: "deposit_verification_mismatch", Title: "Deposit verification mismatch", Message: reason, Fingerprint: "deposit-mismatch:" + deposit.Reference, Metadata: map[string]interface{}{"reference": deposit.Reference}})
			continue
		}
		result := s.wallets.CreditWallet(deposit.Reference, data.Amount)
		if result.Error != nil {
			log.Printf(`{"level":"error","event":"deposit_reconcile_failed","reference":%q}`, deposit.Reference)
		}
	}
	return nil
}

func (s *Service) Status(ctx context.Context) (Control, []Finding, error) {
	var control Control
	err := s.db.WithContext(ctx).FirstOrCreate(&control, Control{ID: 1}).Error
	if err != nil {
		return control, nil, err
	}
	var findings []Finding
	err = s.db.WithContext(ctx).Where("resolved_at IS NULL").Order("created_at DESC").Limit(100).Find(&findings).Error
	return control, findings, err
}

func (s *Service) InspectReference(ctx context.Context, reference string) (*ReferenceInspection, error) {
	reference = strings.TrimSpace(reference)
	result := &ReferenceInspection{Reference: reference, Events: []map[string]interface{}{}}
	var deposit wallets.WalletDeposit
	if err := s.db.WithContext(ctx).Where("reference = ?", reference).First(&deposit).Error; err == nil {
		result.Deposit = &deposit
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	var withdrawal withdrawals.Withdrawal
	if err := s.db.WithContext(ctx).
		Where("reference = ? OR provider_reference = ?", reference, reference).
		First(&withdrawal).Error; err == nil {
		result.Withdrawal = &withdrawal
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Table("paystack_events").
		Select("id, provider_event_id, event_type, status, attempts, last_error, created_at, processed_at").
		Where("resource_reference = ?", reference).
		Order("created_at ASC").Find(&result.Events).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) StartWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				control, _, statusErr := s.Status(ctx)
				if statusErr != nil {
					log.Printf(`{"level":"error","event":"reconciliation_status_failed","error":%q}`, statusErr.Error())
				} else if !control.ProcessingHalted {
					if err := s.ReconcileStuck(ctx); err != nil {
						log.Printf(`{"level":"error","event":"stuck_transaction_reconciliation_failed","error":%q}`, err.Error())
					}
				}
				if _, err := s.RunFinancialCheck(ctx); err != nil {
					log.Printf(`{"level":"error","event":"financial_reconciliation_failed","error":%q}`, err.Error())
				}
			}
		}
	}()
}
