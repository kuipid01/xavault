package withdrawals

import (
	"strings"
	"time"

	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type WithdrawalService struct {
	repo         *WithdrawalRepository
	planRepo     *spendingplans.PlanRepository
	envelopeRepo *envelope.EnvelopeRepo
	walletRepo   *wallets.WalletRepository
}

func NewWithdrawalService(
	repo *WithdrawalRepository,
	planRepo *spendingplans.PlanRepository,
	envelopeRepo *envelope.EnvelopeRepo,
	walletRepo *wallets.WalletRepository,
) *WithdrawalService {
	return &WithdrawalService{
		repo:         repo,
		planRepo:     planRepo,
		envelopeRepo: envelopeRepo,
		walletRepo:   walletRepo,
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
	userID string,
	planID string,
	req RequestWithdrawalRequest,
) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	validationErrors := validateRequestWithdrawalRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	var createdWithdrawal *Withdrawal
	var updatedWallet *wallets.Wallet

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		// Expected behaviour:
		// We recalculate the preview inside the transaction.
		// This prevents the user from previewing one amount, then withdrawing after balances changed.
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

		now := time.Now()

		withdrawal := &Withdrawal{
			ID:          security.GenateUniqueID(),
			UserID:      userID,
			WalletID:    plan.WalletID,
			PlanID:      plan.ID,
			EnvelopeID:  normalizeOptionalString(req.EnvelopeID),
			SourceType:  strings.TrimSpace(req.SourceType),
			Status:      WithdrawalStatusPaid,
			Amount:      req.Amount,
			Currency:    plan.Currency,
			Reason:      strings.TrimSpace(req.Reason),
			Reference:   "WITHDRAWAL_" + security.GenateUniqueID(),
			Description: buildWithdrawalDescription(req.SourceType, req.EnvelopeID),
			PaidAt:      &now,
		}

		err = s.repo.CreateWithTx(tx, withdrawal)
		if err != nil {
			return err
		}

		// Expected behaviour:
		// A withdrawal removes money from locked balance.
		// It also reduces ledger balance because money has left the wallet.
		// Available balance should not increase here.
		wallet, balanceBefore, balanceAfter, err := s.walletRepo.DebitLockedFundsForWithdrawalTx(
			tx,
			userID,
			plan.WalletID,
			req.Amount,
		)
		if err != nil {
			return err
		}

		ledger := &wallets.WalletLedger{
			ID:            security.GenateUniqueID(),
			UserID:        userID,
			WalletID:      plan.WalletID,
			Currency:      wallet.Currency,
			Reference:     "LEDGER_WITHDRAWAL_" + withdrawal.ID,
			Type:          wallets.WalletLedgerTypeDebit,
			Source:        wallets.WalletLedgerSourceWithdrawal,
			SourceID:      withdrawal.ID,
			Amount:        req.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			Description:   "Withdrawal from spending plan",
		}

		_, err = s.walletRepo.CreateWalletLedgerTx(tx, ledger)
		if err != nil {
			return err
		}

		createdWithdrawal = withdrawal
		updatedWallet = wallet

		return nil
	})

	if err != nil {
		return withdrawalErrorResponse("Could not request withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal completed successfully",
		Data: map[string]interface{}{
			"withdrawal": mapWithdrawalToResponse(createdWithdrawal),
			"wallet": WalletSnapshotDTO{
				WalletID:         updatedWallet.ID,
				AvailableBalance: updatedWallet.AvailableBalance,
				LockedBalance:    updatedWallet.LockedBalance,
				LedgerBalance:    updatedWallet.LedgerBalance,
				Currency:         updatedWallet.Currency,
			},
		},
	}
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
