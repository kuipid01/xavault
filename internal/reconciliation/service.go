package reconciliation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
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
}

type RunResult struct {
	RunID            string    `json:"run_id"`
	CheckedAt        time.Time `json:"checked_at"`
	Findings         int       `json:"findings"`
	ProcessingHalted bool      `json:"processing_halted"`
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

	findings = append(findings, s.referenceFindings(runID)...)
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
	}

	return &RunResult{RunID: runID, CheckedAt: now, Findings: len(findings), ProcessingHalted: halt}, nil
}

func (s *Service) referenceFindings(runID string) []Finding {
	var findings []Finding
	type duplicate struct {
		Reference string
		Count     int64
	}
	var ledgerDuplicates []duplicate
	_ = s.db.Model(&wallets.WalletLedger{}).
		Select("reference, COUNT(*) AS count").
		Group("reference").Having("COUNT(*) > 1").Scan(&ledgerDuplicates).Error
	for _, item := range ledgerDuplicates {
		findings = append(findings, Finding{
			RunID: runID, Severity: SeverityCritical, Code: "duplicate_ledger_reference",
			EntityType: "ledger", EntityID: item.Reference, Expected: "1", Actual: fmt.Sprintf("%d", item.Count),
		})
	}
	return findings
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
		reference := strings.TrimSpace(withdrawal.ProviderReference)
		if reference == "" {
			reference = withdrawal.Reference
		}
		verify, err := s.provider.VerifyWithdrawal(ctx, reference)
		if err != nil {
			log.Printf(`{"level":"error","event":"withdrawal_reconcile_provider_error","reference":%q,"error":%q}`, reference, err.Error())
			continue
		}
		var data struct {
			Status    string `json:"status"`
			Reason    string `json:"reason"`
			Reference string `json:"reference"`
		}
		raw, _ := json.Marshal(verify.Data)
		if json.Unmarshal(raw, &data) != nil {
			continue
		}
		switch strings.ToLower(data.Status) {
		case "success":
			result := s.withdrawals.CompleteProviderWithdrawal(reference, data.Status)
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
		verify, err := s.provider.VerifyDeposit(ctx, deposit.Reference)
		if err != nil {
			log.Printf(`{"level":"error","event":"deposit_reconcile_provider_error","reference":%q}`, deposit.Reference)
			continue
		}
		var data struct {
			Status    string `json:"status"`
			Reference string `json:"reference"`
			Amount    int64  `json:"amount"`
			Currency  string `json:"currency"`
		}
		raw, _ := json.Marshal(verify.Data)
		if json.Unmarshal(raw, &data) != nil || strings.ToLower(data.Status) != "success" {
			continue
		}
		if data.Reference != deposit.Reference || data.Amount != deposit.Amount || !strings.EqualFold(data.Currency, deposit.Currency) {
			reason := fmt.Sprintf("deposit verification mismatch for %s", deposit.Reference)
			_ = s.halt(ctx, reason)
			log.Printf(`{"level":"critical","event":"deposit_amount_mismatch","reference":%q}`, deposit.Reference)
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
				if err := s.ReconcileStuck(ctx); err != nil {
					log.Printf(`{"level":"error","event":"stuck_transaction_reconciliation_failed","error":%q}`, err.Error())
				}
				if _, err := s.RunFinancialCheck(ctx); err != nil {
					log.Printf(`{"level":"error","event":"financial_reconciliation_failed","error":%q}`, err.Error())
				}
			}
		}
	}()
}
