package withdrawals

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/envelope"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	httpclient "github.com/kuipid01/salary-vault/internal/shared/client"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const localPendingWithdrawalGracePeriod = 2 * time.Minute

type WithdrawalService struct {
	repo               *WithdrawalRepository
	planRepo           *spendingplans.PlanRepository
	envelopeRepo       *envelope.EnvelopeRepo
	walletRepo         *wallets.WalletRepository
	bankAccountRepo    *bankaccounts.Repository
	userRepo           *users.UserRepository
	payment            paymentproviders.PaymentProvider
	idempotencyManager *client.IdempotentRequest
}

func NewWithdrawalService(
	repo *WithdrawalRepository,
	planRepo *spendingplans.PlanRepository,
	envelopeRepo *envelope.EnvelopeRepo,
	walletRepo *wallets.WalletRepository,
	bankAccountRepo *bankaccounts.Repository,
	userRepo *users.UserRepository,
	payment paymentproviders.PaymentProvider,
	idempotencyManager *client.IdempotentRequest,
) *WithdrawalService {
	return &WithdrawalService{
		repo:               repo,
		planRepo:           planRepo,
		envelopeRepo:       envelopeRepo,
		walletRepo:         walletRepo,
		bankAccountRepo:    bankAccountRepo,
		userRepo:           userRepo,
		payment:            payment,
		idempotencyManager: idempotencyManager,
	}
}

func (s *WithdrawalService) PreviewWithdrawal(
	userID string,
	planID string,
	req PreviewWithdrawalRequest,
) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	validationErrors := validatePreviewWithdrawalRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}
	plan, err := s.planRepo.FindByID(planID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch spending plan",
			Error:   err,
		}
	}

	if plan == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan not found",
			Error:   ErrPlanNotFound,
		}
	}

	if !isPlanWithdrawableStatus(plan.Status) {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan is not active",
			Error:   ErrPlanNotActive,
		}
	}

	if plan.UnboxedBalancePolicy == spendingplans.PlanUnboxedPolicyLockedUntilPlanEnds {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan is locked until the plan ends",
			Error:   ErrSpendingPlanLockedUntilPlanEnds,
		}
	}
	preview, err := s.calculateWithdrawalPreview(userID, planID, req.SourceType, req.EnvelopeID, req.Amount)

	if err != nil {
		return withdrawalErrorResponse("Could not preview withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal preview generated successfully",
		Data:    preview,
	}
}

func (s *WithdrawalService) RequestWithdrawal(
	ctx context.Context,
	userID string,
	email string, // keep this if your signature needs it, but Paystack transfer does not use it
	planID string,
	req RequestWithdrawalRequest,
	idempotencyKey string,
) *response.APIResponse {
	planID = strings.TrimSpace(planID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	if idempotencyKey == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Idempotency key is required",
			Error:   "idempotency_key_required",
		}
	}

	req.SourceType = strings.TrimSpace(req.SourceType)
	req.Reason = strings.TrimSpace(req.Reason)
	req.DestinationType = strings.TrimSpace(req.DestinationType)
	req.DestinationID = strings.TrimSpace(req.DestinationID)
	req.WithdrawalPin = strings.TrimSpace(req.WithdrawalPin)

	validationErrors := validateRequestWithdrawalRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	if err := s.verifyWithdrawalPin(userID, req.WithdrawalPin); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal PIN",
			Error:   err,
		}
	}

	recipientCode, err := s.resolveWithdrawalRecipient(userID, req)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal destination",
			Error:   err.Error(),
		}
	}

	// Data flow:
	// 1. The idempotency manager wraps the whole withdrawal request.
	// 2. This prevents duplicate local withdrawal records and duplicate Paystack transfer attempts.
	// 3. If the same idempotency key is retried after success, the saved response is returned.
	code, payload, err := s.idempotencyManager.HandleIdempotentRequest(
		ctx,
		idempotencyKey,
		userID,
		func() (int, interface{}, error) {
			var createdWithdrawal *Withdrawal
			var updatedWallet *wallets.Wallet
			var planCurrency string

			now := time.Now()

			withdrawalID := security.GenateUniqueID()
			holdLedgerID := security.GenateUniqueID()
			reference := "WITHDRAWAL_" + security.GenateUniqueID()

			// Data flow:
			// 1. Start DB transaction.
			// 2. Lock the spending plan row.
			// 3. Validate plan status and policy.
			// 4. Calculate withdrawable amount using locked DB state.
			// 5. Verify the plan's locked wallet balance can cover the withdrawal.
			// 6. Create withdrawal record with pending status.
			// 7. Create withdrawal hold ledger.
			// 8. Commit transaction before calling Paystack.
			err := s.repo.WithTransaction(func(tx *gorm.DB) error {
				plan, err := s.planRepo.FindByIDAndUserIDForUpdate(tx, planID, userID)
				if err != nil {
					return err
				}

				if plan == nil {
					return ErrPlanNotFound
				}

				if !isPlanWithdrawableStatus(plan.Status) {
					return ErrPlanNotActive
				}

				// Locked plans should only block withdrawal before the plan has ended.
				// If your product rule is stricter, remove the date check.
				if plan.UnboxedBalancePolicy == spendingplans.PlanUnboxedPolicyLockedUntilPlanEnds &&
					now.Before(plan.EndDate) {
					return ErrSpendingPlanLockedUntilPlanEnds
				}

				planCurrency = plan.Currency

				// This preview must use locked rows internally, especially when source type is envelope.
				preview, err := s.calculateWithdrawalPreviewTx(
					tx,
					userID,
					planID,
					req.SourceType,
					req.EnvelopeID,
					req.Amount,
				)
				if err != nil {
					return err
				}
				if !preview.CanWithdraw {
					return ErrInsufficientWithdrawable
				}

				err = s.ensureNoBlockingOpenWithdrawalTx(
					tx,
					userID,
					planID,
					req.SourceType,
					req.EnvelopeID,
					now,
				)
				if err != nil {
					return err
				}

				// Critical money check:
				// Confirmed spending plan funds already live in locked_balance.
				// Keep balances unchanged until Paystack confirms transfer success.
				updatedWallet, err = s.walletRepo.VerifyLockedFundsForWithdrawalTx(
					tx,
					userID,
					plan.WalletID,
					req.Amount,
				)
				if err != nil {
					return err
				}

				withdrawal := &Withdrawal{
					ID:          withdrawalID,
					UserID:      userID,
					WalletID:    plan.WalletID,
					PlanID:      plan.ID,
					EnvelopeID:  normalizeOptionalString(req.EnvelopeID),
					SourceType:  req.SourceType,
					Status:      WithdrawalStatusPending,
					Amount:      req.Amount,
					Currency:    planCurrency,
					Reason:      req.Reason,
					Reference:   reference,
					Description: buildWithdrawalDescription(req.SourceType, req.EnvelopeID),

					PaidAt: nil,

					Provider:            paymentproviders.Paystack,
					ProviderStatus:      "local_pending",
					RecipientCode:       recipientCode,
					DestinationType:     req.DestinationType,
					DestinationID:       req.DestinationID,
					IdempotencyKey:      idempotencyKey,
					RequestedAt:         &now,
					TransferInitiatedAt: nil,
				}

				if err := s.repo.CreateWithTx(tx, withdrawal); err != nil {
					return err
				}

				createdWithdrawal = withdrawal

				holdLedger := &wallets.WalletLedger{
					ID:        holdLedgerID,
					UserID:    userID,
					WalletID:  plan.WalletID,
					Currency:  plan.Currency,
					Reference: reference,
					Source:    wallets.WalletLedgerSourceWithdrawal,
					SourceID:  withdrawal.ID,
					Type:      wallets.WalletLedgerTypeWithdrawalHold,

					Amount:    req.Amount,
					CreatedAt: now,
				}

				_, err = s.walletRepo.CreateWalletLedgerTx(tx, holdLedger)
				if err != nil {
					return err
				}

				return nil
			})

			if err != nil {
				return http.StatusBadRequest, nil, err
			}

			// Data flow:
			// 1. Local DB now has a pending withdrawal and locked funds.
			// 2. Now call Paystack.
			// 3. Initiation does not mean success.
			// 4. Save Paystack transfer_code/reference/status.
			// 5. Webhook will later mark withdrawal as success or failed.
			amountForPaystack := req.Amount

			transferReason := req.Reason
			if transferReason == "" {
				transferReason = "Withdrawal from spending plan"
			}

			paystackReq := &paymentproviders.InitiateTransferBody{
				Source:    "balance",
				Amount:    amountForPaystack,
				Recipient: recipientCode,
				Reason:    transferReason,
				Reference: createdWithdrawal.Reference,
			}

			paystackResp, err := s.payment.InitiateTransfer(ctx, paystackReq)
			if err != nil {
				// Data flow:
				// Paystack initiation failed.
				// No wallet balance was moved locally, so only mark the request failed.
				reversalErr := s.markFailedWithdrawalInitiation(
					createdWithdrawal.ID,
					userID,
					createdWithdrawal.WalletID,
					createdWithdrawal.Amount,
					createdWithdrawal.Currency,
					createdWithdrawal.Reference,
					fmt.Sprintf("Paystack transfer initiation failed: %v", err),
				)
				if reversalErr != nil {
					return http.StatusInternalServerError, nil, fmt.Errorf(
						"paystack failed: %w; reversal also failed: %v",
						err,
						reversalErr,
					)
				}

				return http.StatusBadRequest, nil, err
			}

			// Data flow:
			// Paystack accepted the transfer initiation.
			// We now save provider data.
			// Do not mark as paid here.
			err = s.repo.WithTransaction(func(tx *gorm.DB) error {
				return s.repo.MarkTransferInitiatedWithTx(
					tx,
					createdWithdrawal.ID,
					paystackResp.Data.Reference,
					paystackResp.Data.TransferCode,
					paystackResp.Data.Status,
					paystackResp.Data,
				)
			})
			if err != nil {
				// This is serious because Paystack may process the transfer but DB failed to save transfer code.
				// Keep the idempotency as failed so the request can be investigated/retried carefully.
				return http.StatusInternalServerError, nil, fmt.Errorf("failed to save Paystack transfer data: %w", err)
			}

			return http.StatusAccepted, map[string]interface{}{
				"message":    "Withdrawal request submitted and transfer processing started",
				"withdrawal": mapWithdrawalToResponse(createdWithdrawal),
				"wallet": WalletSnapshotDTO{
					WalletID:         updatedWallet.ID,
					AvailableBalance: updatedWallet.AvailableBalance,
					LockedBalance:    updatedWallet.LockedBalance,
					LedgerBalance:    updatedWallet.LedgerBalance,
					Currency:         updatedWallet.Currency,
				},
				"paystack": paystackResp.Data,
				"note":     "Final withdrawal success will be confirmed by Paystack webhook",
			}, nil
		},
	)

	if err != nil {
		return withdrawalErrorResponse("Could not request withdrawal", err)
	}

	if code >= 400 {
		return &response.APIResponse{
			Success: false,
			Message: "Could not request withdrawal",
			Error:   payload,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal request submitted successfully",
		Data:    payload,
	}
}

func (s *WithdrawalService) markFailedWithdrawalInitiation(
	withdrawalID string,
	userID string,
	walletID string,
	amount int64,
	currency string,
	reference string,
	reason string,
) error {
	now := time.Now()

	return s.repo.WithTransaction(func(tx *gorm.DB) error {
		// Data flow:
		// 1. Paystack failed before final transfer processing.
		// 2. Mark withdrawal as failed.
		// 3. Write a ledger event for auditability; balances do not change.
		err := s.repo.MarkFailedWithTx(tx, withdrawalID, reason)
		if err != nil {
			return err
		}

		reversalLedger := &wallets.WalletLedger{
			ID:        security.GenateUniqueID(),
			UserID:    userID,
			WalletID:  walletID,
			Currency:  currency,
			Reference: "WITHDRAWAL_FAILED_" + withdrawalID,

			Source:   wallets.WalletLedgerSourceWithdrawal,
			SourceID: withdrawalID,
			Type:     wallets.WalletLedgerTypeWithdrawalReversal,

			Amount:    amount,
			CreatedAt: now,
		}

		_, err = s.walletRepo.CreateWalletLedgerTx(tx, reversalLedger)
		return err
	})
}
func (s *WithdrawalService) GetPlanWithdrawals(
	userID string,
	planID string,
) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	plan, err := s.planRepo.FindByIDAndUserID(planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch spending plan",
			Error:   err,
		}
	}

	if plan == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan not found",
			Error:   ErrPlanNotFound,
		}
	}

	withdrawals, err := s.repo.FindByPlanIDAndUserID(planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch withdrawals",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawals fetched successfully",
		Data: ListWithdrawalsResponse{
			Items: mapWithdrawalsToResponse(withdrawals),
		},
	}
}

func (s *WithdrawalService) ensureNoBlockingOpenWithdrawalTx(
	tx *gorm.DB,
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
	now time.Time,
) error {
	openWithdrawal, err := s.repo.FindOpenWithdrawalForSourceTx(
		tx,
		userID,
		planID,
		sourceType,
		normalizeOptionalString(envelopeID),
	)
	if err != nil {
		return err
	}

	if openWithdrawal == nil {
		return nil
	}

	if openWithdrawal.Status == WithdrawalStatusProcessing ||
		openWithdrawal.TransferInitiatedAt != nil ||
		openWithdrawal.ProviderTransferCode != "" {
		return ErrWithdrawalAlreadyProcessing
	}

	if now.Sub(openWithdrawal.CreatedAt) < localPendingWithdrawalGracePeriod {
		return ErrWithdrawalRequestInProgress
	}

	return s.repo.MarkFailedWithTx(
		tx,
		openWithdrawal.ID,
		fmt.Sprintf(
			"stale local pending withdrawal expired after %s without Paystack transfer initiation",
			localPendingWithdrawalGracePeriod,
		),
	)
}

func (s *WithdrawalService) GetWithdrawalByID(
	userID string,
	withdrawalID string,
) *response.APIResponse {
	withdrawalID = strings.TrimSpace(withdrawalID)

	if withdrawalID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal ID is required",
			Error:   "withdrawal_id_required",
		}
	}

	withdrawal, err := s.repo.FindByIDAndUserID(withdrawalID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch withdrawal",
			Error:   err,
		}
	}

	if withdrawal == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal not found",
			Error:   ErrWithdrawalNotFound,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal fetched successfully",
		Data:    mapWithdrawalToResponse(withdrawal),
	}
}

func (s *WithdrawalService) CompleteProviderWithdrawal(reference string, providerStatus string) *response.APIResponse {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal reference is required",
			Error:   "withdrawal_reference_required",
		}
	}

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		withdrawal, err := s.repo.FindByReferenceForUpdate(tx, reference)
		if err != nil {
			return err
		}

		if withdrawal == nil {
			return ErrWithdrawalNotFound
		}

		if withdrawal.Status == WithdrawalStatusPaid {
			return nil
		}

		if withdrawal.Status == WithdrawalStatusFailed || withdrawal.Status == WithdrawalStatusCancelled {
			return fmt.Errorf("withdrawal is already final with status %s", withdrawal.Status)
		}

		lockedBefore, updatedWallet, err := s.walletRepo.CompleteWithdrawalTx(
			tx,
			withdrawal.UserID,
			withdrawal.WalletID,
			withdrawal.Amount,
		)
		if err != nil {
			return err
		}

		if err := s.repo.MarkPaidWithTx(tx, withdrawal.ID, providerStatus); err != nil {
			return err
		}

		ledger := &wallets.WalletLedger{
			ID:            security.GenateUniqueID(),
			UserID:        withdrawal.UserID,
			WalletID:      withdrawal.WalletID,
			Currency:      withdrawal.Currency,
			Reference:     "WITHDRAWAL_PAID_" + withdrawal.ID,
			Source:        wallets.WalletLedgerSourceWithdrawal,
			SourceID:      withdrawal.ID,
			Type:          wallets.WalletLedgerTypeWithdrawalDebit,
			Amount:        withdrawal.Amount,
			BalanceBefore: lockedBefore,
			BalanceAfter:  updatedWallet.LockedBalance,
			Description:   "Withdrawal completed by Paystack",
			CreatedAt:     time.Now(),
		}

		_, err = s.walletRepo.CreateWalletLedgerTx(tx, ledger)
		return err
	})

	if err != nil {
		return withdrawalErrorResponse("Could not complete withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal completed successfully",
	}
}

func (s *WithdrawalService) FailProviderWithdrawal(reference string, providerStatus string, reason string) *response.APIResponse {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal reference is required",
			Error:   "withdrawal_reference_required",
		}
	}

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		withdrawal, err := s.repo.FindByReferenceForUpdate(tx, reference)
		if err != nil {
			return err
		}

		if withdrawal == nil {
			return ErrWithdrawalNotFound
		}

		if withdrawal.Status == WithdrawalStatusPaid ||
			withdrawal.Status == WithdrawalStatusFailed ||
			withdrawal.Status == WithdrawalStatusCancelled {
			return nil
		}

		if reason == "" {
			reason = "Paystack transfer " + providerStatus
		}

		return s.repo.MarkFailedWithTx(tx, withdrawal.ID, reason)
	})

	if err != nil {
		return withdrawalErrorResponse("Could not fail withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal marked failed successfully",
	}
}

func (s *WithdrawalService) calculateWithdrawalPreview(
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
	amount int64,
) (*PreviewWithdrawalResponse, error) {
	return s.calculateWithdrawalPreviewTx(nil, userID, planID, sourceType, envelopeID, amount)
}

func (s *WithdrawalService) calculateWithdrawalPreviewTx(
	tx *gorm.DB,
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
	amount int64,
) (*PreviewWithdrawalResponse, error) {
	log.Printf(
		"[withdrawal_preview] start userID=%s planID=%s sourceType=%q envelopeID=%v amount=%d",
		userID,
		planID,
		sourceType,
		envelopeID,
		amount,
	)

	sourceType = strings.TrimSpace(sourceType)

	log.Printf(
		"[withdrawal_preview] normalized input userID=%s planID=%s sourceType=%q envelopeID=%v amount=%d",
		userID,
		planID,
		sourceType,
		envelopeID,
		amount,
	)

	if amount <= 0 {
		log.Printf(
			"[withdrawal_preview] invalid amount userID=%s planID=%s amount=%d",
			userID,
			planID,
			amount,
		)
		return nil, ErrInvalidWithdrawalAmount
	}

	plan, err := s.findPlanForWithdrawal(tx, planID, userID)
	if err != nil {
		log.Printf(
			"[withdrawal_preview] failed to find plan userID=%s planID=%s err=%v",
			userID,
			planID,
			err,
		)
		return nil, err
	}

	if plan == nil {
		log.Printf(
			"[withdrawal_preview] plan not found userID=%s planID=%s",
			userID,
			planID,
		)
		return nil, ErrPlanNotFound
	}

	log.Printf(
		"[withdrawal_preview] plan found userID=%s planID=%s planStatus=%s planAmount=%d",
		userID,
		plan.ID,
		plan.Status,
		plan.Amount,
	)

	if !isPlanWithdrawableStatus(plan.Status) {
		log.Printf(
			"[withdrawal_preview] plan not withdrawable userID=%s planID=%s planStatus=%s",
			userID,
			plan.ID,
			plan.Status,
		)
		return nil, ErrPlanNotActive
	}

	var withdrawableBalance int64

	switch sourceType {
	case WithdrawalSourcePlanUnboxed:
		log.Printf(
			"[withdrawal_preview] calculating plan unboxed withdrawable userID=%s planID=%s",
			userID,
			plan.ID,
		)

		if envelopeID != nil && strings.TrimSpace(*envelopeID) != "" {
			log.Printf(
				"[withdrawal_preview] envelope not allowed for unboxed withdrawal userID=%s planID=%s envelopeID=%q",
				userID,
				plan.ID,
				*envelopeID,
			)
			return nil, ErrEnvelopeNotAllowed
		}

		withdrawableBalance, err = s.calculatePlanUnboxedWithdrawable(tx, plan.ID, userID, plan.Amount)
		if err != nil {
			log.Printf(
				"[withdrawal_preview] failed to calculate plan unboxed withdrawable userID=%s planID=%s err=%v",
				userID,
				plan.ID,
				err,
			)
			return nil, err
		}

		log.Printf(
			"[withdrawal_preview] calculated plan unboxed withdrawable userID=%s planID=%s withdrawableBalance=%d",
			userID,
			plan.ID,
			withdrawableBalance,
		)

	case WithdrawalSourceEnvelope:
		normalizedEnvelopeID := normalizeOptionalString(envelopeID)

		log.Printf(
			"[withdrawal_preview] calculating envelope withdrawable userID=%s planID=%s envelopeID=%v",
			userID,
			plan.ID,
			normalizedEnvelopeID,
		)

		if normalizedEnvelopeID == nil {
			log.Printf(
				"[withdrawal_preview] envelope required userID=%s planID=%s",
				userID,
				plan.ID,
			)
			return nil, ErrEnvelopeRequired
		}

		withdrawableBalance, err = s.calculateEnvelopeWithdrawable(tx, plan.ID, userID, *normalizedEnvelopeID)
		if err != nil {
			log.Printf(
				"[withdrawal_preview] failed to calculate envelope withdrawable userID=%s planID=%s envelopeID=%s err=%v",
				userID,
				plan.ID,
				*normalizedEnvelopeID,
				err,
			)
			return nil, err
		}

		log.Printf(
			"[withdrawal_preview] calculated envelope withdrawable userID=%s planID=%s envelopeID=%s withdrawableBalance=%d",
			userID,
			plan.ID,
			*normalizedEnvelopeID,
			withdrawableBalance,
		)

	default:
		log.Printf(
			"[withdrawal_preview] invalid withdrawal source userID=%s planID=%s sourceType=%q",
			userID,
			plan.ID,
			sourceType,
		)
		return nil, ErrInvalidWithdrawalSource
	}

	validationErrors := []string{}

	if amount > withdrawableBalance {
		log.Printf(
			"[withdrawal_preview] insufficient withdrawable balance userID=%s planID=%s sourceType=%s amount=%d withdrawableBalance=%d",
			userID,
			plan.ID,
			sourceType,
			amount,
			withdrawableBalance,
		)

		validationErrors = append(validationErrors, "Insufficient withdrawable balance")
	}

	canWithdraw := len(validationErrors) == 0
	remainingBalance := withdrawableBalance - amount

	log.Printf(
		"[withdrawal_preview] completed userID=%s planID=%s sourceType=%s envelopeID=%v amount=%d withdrawableBalance=%d remainingBalance=%d canWithdraw=%t validationErrors=%v",
		userID,
		plan.ID,
		sourceType,
		normalizeOptionalString(envelopeID),
		amount,
		withdrawableBalance,
		remainingBalance,
		canWithdraw,
		validationErrors,
	)

	return &PreviewWithdrawalResponse{
		CanWithdraw:         canWithdraw,
		SourceType:          sourceType,
		PlanID:              plan.ID,
		EnvelopeID:          normalizeOptionalString(envelopeID),
		RequestedAmount:     amount,
		WithdrawableBalance: withdrawableBalance,
		RemainingBalance:    remainingBalance,
		ValidationErrors:    validationErrors,
	}, nil
}

func (s *WithdrawalService) calculatePlanUnboxedWithdrawable(
	tx *gorm.DB,
	planID string,
	userID string,
	planAmount int64,
) (int64, error) {
	// Expected behaviour:
	// Plan unboxed balance is the part of the plan not assigned to top-level envelopes.
	// Previous plan-level withdrawals must also be removed from that balance.

	topLevelAllocated, err := s.envelopeRepo.SumTopLevelEnvelopeAmount(planID)
	if err != nil {
		return 0, err
	}

	previousWithdrawals, err := s.repo.SumPaidPlanUnboxedWithdrawalsTx(tx, planID, userID)
	if err != nil {
		return 0, err
	}

	withdrawable := planAmount - topLevelAllocated - previousWithdrawals

	if withdrawable < 0 {
		return 0, nil
	}

	return withdrawable, nil
}

func (s *WithdrawalService) calculateEnvelopeWithdrawable(
	tx *gorm.DB,
	planID string,
	userID string,
	envelopeID string,
) (int64, error) {
	// Expected behaviour:
	// Envelope withdrawable balance is only the envelope's own unboxed balance.
	// Child envelope balances must not be drained by parent withdrawals.

	env, err := s.findEnvelopeForWithdrawal(tx, envelopeID, planID, userID)
	if err != nil {
		return 0, err
	}

	if env == nil {
		return 0, ErrEnvelopeNotFound
	}

	childAllocated, err := s.envelopeRepo.SumChildEnvelopeAmountTx(tx, env.ID)
	if err != nil {
		return 0, err
	}

	previousWithdrawals, err := s.repo.SumPaidEnvelopeWithdrawalsTx(tx, env.ID, userID)
	if err != nil {
		return 0, err
	}

	withdrawable := env.Amount - childAllocated - previousWithdrawals

	if withdrawable < 0 {
		return 0, nil
	}

	return withdrawable, nil
}

func (s *WithdrawalService) findPlanForWithdrawal(
	tx *gorm.DB,
	planID string,
	userID string,
) (*spendingplans.SpendingPlan, error) {
	if tx != nil {
		return s.planRepo.FindByIDAndUserIDForUpdate(tx, planID, userID)
	}

	return s.planRepo.FindByIDAndUserID(planID, userID)
}

func (s *WithdrawalService) findEnvelopeForWithdrawal(
	tx *gorm.DB,
	envelopeID string,
	planID string,
	userID string,
) (*envelope.Envelope, error) {
	if tx != nil {
		return s.envelopeRepo.FindByIDPlanIDAndUserIDForUpdate(tx, envelopeID, planID, userID)
	}

	return s.envelopeRepo.FindByIDPlanIDAndUserID(envelopeID, planID, userID)
}

func validatePreviewWithdrawalRequest(req PreviewWithdrawalRequest) map[string]string {
	errors := map[string]string{}

	sourceType := strings.TrimSpace(req.SourceType)

	if sourceType == "" {
		errors["source_type"] = "Source type is required"
	}

	if sourceType != "" && sourceType != WithdrawalSourcePlanUnboxed && sourceType != WithdrawalSourceEnvelope {
		errors["source_type"] = "Invalid source type"
	}

	if req.Amount <= 0 {
		errors["amount"] = "Amount must be greater than zero"
	}

	if sourceType == WithdrawalSourceEnvelope {
		if req.EnvelopeID == nil || strings.TrimSpace(*req.EnvelopeID) == "" {
			errors["envelope_id"] = "Envelope ID is required"
		}
	}

	if sourceType == WithdrawalSourcePlanUnboxed {
		if req.EnvelopeID != nil && strings.TrimSpace(*req.EnvelopeID) != "" {
			errors["envelope_id"] = "Envelope ID is not allowed for plan unboxed withdrawal"
		}
	}

	return errors
}

func isPlanWithdrawableStatus(status string) bool {
	return status == spendingplans.PlanStatusActive ||
		status == spendingplans.PlanStatusExpired ||
		status == spendingplans.PlanStatusCompleted
}

func validateRequestWithdrawalRequest(req RequestWithdrawalRequest) map[string]string {
	errors := map[string]string{}

	previewErrors := validatePreviewWithdrawalRequest(PreviewWithdrawalRequest{
		SourceType: req.SourceType,
		EnvelopeID: req.EnvelopeID,
		Amount:     req.Amount,
	})

	for key, value := range previewErrors {
		errors[key] = value
	}

	if len(strings.TrimSpace(req.Reason)) > 255 {
		errors["reason"] = "Reason cannot be more than 255 characters"
	}

	destinationType := strings.TrimSpace(req.DestinationType)
	if destinationType == "" {
		errors["destination_type"] = "Destination type is required"
	}

	if destinationType != "" &&
		destinationType != WithdrawalDestinationAccount &&
		destinationType != WithdrawalDestinationBeneficiary {
		errors["destination_type"] = "Invalid destination type"
	}

	if strings.TrimSpace(req.DestinationID) == "" {
		errors["destination_id"] = "Destination ID is required"
	}

	if pinErrors := validateWithdrawalPinFormat(req.WithdrawalPin); len(pinErrors) > 0 {
		for key, value := range pinErrors {
			errors[key] = value
		}
	}

	return errors
}

func validateWithdrawalPinFormat(pin string) map[string]string {
	errors := map[string]string{}
	pin = strings.TrimSpace(pin)

	if len(pin) != 4 {
		errors["withdrawal_pin"] = "Withdrawal PIN must be 4 digits"
		return errors
	}

	for _, r := range pin {
		if r < '0' || r > '9' {
			errors["withdrawal_pin"] = "Withdrawal PIN must contain only digits"
			return errors
		}
	}

	return errors
}

func (s *WithdrawalService) verifyWithdrawalPin(userID string, pin string) error {
	user, err := s.userRepo.FindById(userID)
	if err != nil {
		return err
	}

	if user == nil || user.WithdrawalPinHash == nil || strings.TrimSpace(*user.WithdrawalPinHash) == "" {
		return ErrWithdrawalPinNotSet
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.WithdrawalPinHash), []byte(pin)); err != nil {
		return ErrInvalidWithdrawalPin
	}

	return nil
}

func (s *WithdrawalService) resolveWithdrawalRecipient(userID string, req RequestWithdrawalRequest) (string, error) {
	ownerType := bankaccounts.AccountOwnerTypeSelf
	if req.DestinationType == WithdrawalDestinationBeneficiary {
		ownerType = bankaccounts.AccountOwnerTypeBeneficiary
	}

	account, err := s.bankAccountRepo.FindByIDAndUserID(req.DestinationID, userID)
	if err != nil {
		return "", err
	}

	if account == nil || account.OwnerType != ownerType {
		return "", ErrInvalidWithdrawalDestination
	}

	if account.RecipientCode == "" {
		return "", ErrInvalidWithdrawalDestination
	}

	return account.RecipientCode, nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func buildWithdrawalDescription(sourceType string, envelopeID *string) string {
	if sourceType == WithdrawalSourceEnvelope && envelopeID != nil {
		return "Withdrawal from envelope"
	}

	return "Withdrawal from plan unboxed balance"
}

func withdrawalErrorResponse(message string, err error) *response.APIResponse {
	if err == ErrWithdrawalNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal not found",
			Error:   err,
		}
	}

	if err == ErrPlanNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan not found",
			Error:   err,
		}
	}

	if err == ErrPlanNotActive {
		return &response.APIResponse{
			Success: false,
			Message: "Only active plans can be withdrawn from",
			Error:   err,
		}
	}

	if err == ErrEnvelopeNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope not found",
			Error:   err,
		}
	}

	if err == ErrInvalidWithdrawalSource {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal source",
			Error:   err,
		}
	}

	if err == ErrInvalidWithdrawalAmount {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal amount",
			Error:   err,
		}
	}

	if err == ErrEnvelopeRequired {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   err,
		}
	}

	if err == ErrEnvelopeNotAllowed {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope ID is not allowed for plan unboxed withdrawal",
			Error:   err,
		}
	}

	if err == ErrInsufficientWithdrawable {
		return &response.APIResponse{
			Success: false,
			Message: "Insufficient withdrawable balance",
			Error:   err,
		}
	}

	if err == ErrWalletNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Wallet not found",
			Error:   err,
		}
	}

	if err == ErrInvalidWithdrawalDestination {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal destination",
			Error:   err,
		}
	}

	if err == ErrWithdrawalRequestInProgress {
		return &response.APIResponse{
			Success: false,
			Message: "A withdrawal request for this source is still being prepared. Please wait a moment.",
			Error:   err,
		}
	}

	if err == ErrWithdrawalAlreadyProcessing {
		return &response.APIResponse{
			Success: false,
			Message: "A withdrawal from this source is already processing with Paystack.",
			Error:   err,
		}
	}

	if err == ErrWithdrawalPinNotSet {
		return &response.APIResponse{
			Success: false,
			Message: "Set your withdrawal PIN before withdrawing",
			Error:   err,
		}
	}

	if err == ErrInvalidWithdrawalPin {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal PIN",
			Error:   err,
		}
	}

	var externalErr *httpclient.ExternalAPIError
	if errors.As(err, &externalErr) {
		message := externalErr.Message
		if message == "" {
			message = "Payment provider rejected the request"
		}

		return &response.APIResponse{
			Success: false,
			Message: message,
			Error: map[string]interface{}{
				"provider":    paymentproviders.Paystack,
				"status_code": externalErr.StatusCode,
				"code":        externalErr.Code,
				"type":        externalErr.Type,
			},
		}
	}

	return &response.APIResponse{
		Success: false,
		Message: message,
		Error:   err,
	}
}
