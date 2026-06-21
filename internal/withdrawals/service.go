package withdrawals

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kuipid01/salary-vault/internal/envelope"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type WithdrawalService struct {
	repo               *WithdrawalRepository
	planRepo           *spendingplans.PlanRepository
	envelopeRepo       *envelope.EnvelopeRepo
	walletRepo         *wallets.WalletRepository
	payment            paymentproviders.PaymentProvider
	idempotencyManager *client.IdempotentRequest
}

func NewWithdrawalService(
	repo *WithdrawalRepository,
	planRepo *spendingplans.PlanRepository,
	envelopeRepo *envelope.EnvelopeRepo,
	walletRepo *wallets.WalletRepository,
	payment paymentproviders.PaymentProvider,
	idempotencyManager *client.IdempotentRequest,
) *WithdrawalService {
	return &WithdrawalService{
		repo:               repo,
		planRepo:           planRepo,
		envelopeRepo:       envelopeRepo,
		walletRepo:         walletRepo,
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

	if plan.Status != spendingplans.PlanStatusActive {
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
	req.RecipientCode = strings.TrimSpace(req.RecipientCode)

	validationErrors := validateRequestWithdrawalRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	if req.RecipientCode == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Recipient code is required",
			Error: map[string]string{
				"recipient_code": "Recipient code is required for Paystack transfer",
			},
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
			// 5. Move amount from wallet available balance to locked balance.
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

				if plan.Status != spendingplans.PlanStatusActive {
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

				// Optional but recommended:
				// Block duplicate pending withdrawal against the same plan/source/envelope.
				exists, err := s.repo.HasPendingWithdrawalForSourceTx(
					tx,
					userID,
					planID,
					req.SourceType,
					req.EnvelopeID,
				)
				if err != nil {
					return err
				}

				if exists {
					return ErrPendingWithdrawalExists
				}

				// Critical money movement:
				// This must lock the wallet row and move:
				// available_balance -= req.Amount
				// locked_balance += req.Amount
				updatedWallet, err = s.walletRepo.HoldFundsForWithdrawalTx(
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
					RecipientCode:       req.RecipientCode,
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
				Recipient: req.RecipientCode,
				Reason:    transferReason,
				Reference: createdWithdrawal.Reference,
			}

			paystackResp, err := s.payment.InitiateTransfer(ctx, paystackReq)
			if err != nil {
				// Data flow:
				// Paystack initiation failed.
				// Reverse the local fund hold so user is not stuck.
				reversalErr := s.reverseFailedWithdrawalHold(
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

func (s *WithdrawalService) reverseFailedWithdrawalHold(
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
		// 2. Release locked withdrawal funds back to available balance.
		// 3. Mark withdrawal as failed.
		// 4. Write reversal ledger.
		updatedWallet, err := s.walletRepo.ReleaseWithdrawalHoldTx(
			tx,
			userID,
			walletID,
			amount,
		)
		if err != nil {
			return err
		}

		_ = updatedWallet

		err = s.repo.MarkFailedWithTx(tx, withdrawalID, reason)
		if err != nil {
			return err
		}

		reversalLedger := &wallets.WalletLedger{
			ID:        security.GenateUniqueID(),
			UserID:    userID,
			WalletID:  walletID,
			Currency:  currency,
			Reference: reference,

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
	sourceType = strings.TrimSpace(sourceType)

	if amount <= 0 {
		return nil, ErrInvalidWithdrawalAmount
	}

	plan, err := s.findPlanForWithdrawal(tx, planID, userID)
	if err != nil {
		return nil, err
	}

	if plan == nil {
		return nil, ErrPlanNotFound
	}

	if plan.Status != spendingplans.PlanStatusActive {
		return nil, ErrPlanNotActive
	}

	var withdrawableBalance int64

	switch sourceType {
	case WithdrawalSourcePlanUnboxed:
		if envelopeID != nil && strings.TrimSpace(*envelopeID) != "" {
			return nil, ErrEnvelopeNotAllowed
		}

		withdrawableBalance, err = s.calculatePlanUnboxedWithdrawable(tx, plan.ID, userID, plan.Amount)
		if err != nil {
			return nil, err
		}

	case WithdrawalSourceEnvelope:
		normalizedEnvelopeID := normalizeOptionalString(envelopeID)
		if normalizedEnvelopeID == nil {
			return nil, ErrEnvelopeRequired
		}

		withdrawableBalance, err = s.calculateEnvelopeWithdrawable(tx, plan.ID, userID, *normalizedEnvelopeID)
		if err != nil {
			return nil, err
		}

	default:
		return nil, ErrInvalidWithdrawalSource
	}

	validationErrors := []string{}

	if amount > withdrawableBalance {
		validationErrors = append(validationErrors, "Insufficient withdrawable balance")
	}

	canWithdraw := len(validationErrors) == 0

	return &PreviewWithdrawalResponse{
		CanWithdraw:         canWithdraw,
		SourceType:          sourceType,
		PlanID:              plan.ID,
		EnvelopeID:          normalizeOptionalString(envelopeID),
		RequestedAmount:     amount,
		WithdrawableBalance: withdrawableBalance,
		RemainingBalance:    withdrawableBalance - amount,
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

	return errors
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

	return &response.APIResponse{
		Success: false,
		Message: message,
		Error:   err,
	}
}
