package spendingplans

import (
	"log"
	"strings"
	"time"

	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type PlanService struct {
	repository *PlanRepository
	walletRepo *wallets.WalletRepository
}

func NewPlanService(r *PlanRepository, walletRepo *wallets.WalletRepository) *PlanService {
	return &PlanService{repository: r, walletRepo: walletRepo}
}

func (s *PlanService) CreateDraftPlan(userID string, req CreatePlanRequest) *response.APIResponse {
	req.Name = strings.TrimSpace(req.Name)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	wallet, walletErr := s.walletRepo.GetWalletByOwnerID(userID)
	if walletErr != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Couldnt get wallet",
			Error:   walletErr,
		}
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	if req.UnboxedBalancePolicy == "" {
		req.UnboxedBalancePolicy = PlanUnboxedPolicyFlexible
	}

	validationErrors := ValidateCreatePlanRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	startDate, err := ParseDate(req.StartDate)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid start date",
			Error: map[string]string{
				"start_date": "Start date must be in YYYY-MM-DD format",
			},
		}
	}

	endDate, err := ParseDate(req.EndDate)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid end date",
			Error: map[string]string{
				"end_date": "End date must be in YYYY-MM-DD format",
			},
		}
	}

	if endDate.Before(startDate) {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid date range",
			Error: map[string]string{
				"end_date": "End date must be after start date",
			},
		}
	}

	plan := &SpendingPlan{
		UserID:               userID,
		WalletID:             wallet.ID,
		Name:                 req.Name,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               PlanStatusDraft,
		UnboxedBalancePolicy: req.UnboxedBalancePolicy,
		StartDate:            startDate,
		EndDate:              endDate,
	}

	err = s.repository.Create(plan)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not create spending plan",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Draft spending plan created successfully",
		Data:    mapPlanToResponse(plan, 0),
	}
}

func (s *PlanService) GetMyPlans(userID string, query ListPlansQuery) *response.APIResponse {
	if query.Page <= 0 {
		query.Page = 1
	}

	if query.Limit <= 0 {
		query.Limit = 20
	}

	if query.Limit > 100 {
		query.Limit = 100
	}

	query.Status = strings.TrimSpace(strings.ToLower(query.Status))

	plans, total, err := s.repository.FindAllByUserID(
		userID,
		query.Status,
		query.Page,
		query.Limit,
	)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch spending plans",
			Error:   err,
		}
	}

	items := make([]PlanResponse, 0, len(plans))

	for _, plan := range plans {
		// For now, allocated amount is 0 until envelope repository is connected.
		items = append(items, mapPlanToResponse(&plan, 0))
	}

	return &response.APIResponse{
		Success: true,
		Message: "Spending plans fetched successfully",
		Data: ListPlansResponse{
			Items: items,
			Pagination: PaginationDTO{
				Page:  query.Page,
				Limit: query.Limit,
				Total: total,
			},
		},
	}
}

func (s *PlanService) GetPlanByID(userID string, planID string) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	plan, err := s.repository.FindByIDAndUserID(planID, userID)
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

	return &response.APIResponse{
		Success: true,
		Message: "Spending plan fetched successfully",
		Data:    mapPlanToResponse(plan, 0),
	}
}

func (s *PlanService) UpdateDraftPlan(userID string, planID string, req UpdatePlanRequest) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	updates := make(map[string]interface{})

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)

		if len(name) < 2 {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid data",
				Error: map[string]string{
					"name": "Name must be at least 2 characters",
				},
			}
		}

		updates["name"] = name
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid data",
				Error: map[string]string{
					"amount": "Amount must be greater than zero",
				},
			}
		}

		updates["amount"] = *req.Amount
	}

	if req.UnboxedBalancePolicy != nil {
		policy := strings.TrimSpace(*req.UnboxedBalancePolicy)

		if !IsValidPlanUnboxedPolicy(policy) {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid data",
				Error: map[string]string{
					"unboxed_balance_policy": "Invalid unboxed balance policy",
				},
			}
		}

		updates["unboxed_balance_policy"] = policy
	}

	if req.StartDate != nil {
		startDate, err := ParseDate(*req.StartDate)
		if err != nil {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid start date",
				Error: map[string]string{
					"start_date": "Start date must be in YYYY-MM-DD format",
				},
			}
		}

		updates["start_date"] = startDate
	}

	if req.EndDate != nil {
		endDate, err := ParseDate(*req.EndDate)
		if err != nil {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid end date",
				Error: map[string]string{
					"end_date": "End date must be in YYYY-MM-DD format",
				},
			}
		}

		updates["end_date"] = endDate
	}

	plan, err := s.repository.UpdateDraftPlan(planID, userID, updates)
	if err != nil {
		if err == ErrPlanNotDraft {
			return &response.APIResponse{
				Success: false,
				Message: "Only draft plans can be updated",
				Error:   err,
			}
		}

		return &response.APIResponse{
			Success: false,
			Message: "Could not update spending plan",
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

	if plan.EndDate.Before(plan.StartDate) {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid date range",
			Error: map[string]string{
				"end_date": "End date must be after start date",
			},
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Draft spending plan updated successfully",
		Data:    mapPlanToResponse(plan, 0),
	}
}

func (s *PlanService) PreviewPlan(userID string, planID string) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	plan, err := s.repository.FindByIDAndUserID(planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not preview spending plan",
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

	validationErrors := []PlanValidationError{}

	if plan.Status != PlanStatusDraft {
		validationErrors = append(validationErrors, PlanValidationError{
			Code:    "PLAN_NOT_DRAFT",
			Message: "Only draft plans can be previewed for confirmation.",
			Field:   "status",
		})
	}

	if plan.Amount <= 0 {
		validationErrors = append(validationErrors, PlanValidationError{
			Code:    "INVALID_PLAN_AMOUNT",
			Message: "Plan amount must be greater than zero.",
			Field:   "amount",
		})
	}

	if plan.EndDate.Before(plan.StartDate) {
		validationErrors = append(validationErrors, PlanValidationError{
			Code:    "INVALID_PLAN_DATE_RANGE",
			Message: "Plan end date must be after start date.",
			Field:   "end_date",
		})
	}

	// Envelope calculation will be connected when envelope module is ready.
	topLevelAllocated := int64(0)
	planUnboxedBalance := plan.Amount - topLevelAllocated

	canConfirm := len(validationErrors) == 0
	wallet, walletErr := s.walletRepo.GetWalletByOwnerID(userID)
	if walletErr != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch wallet",
			Error:   walletErr,
		}
	}

	if wallet == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Wallet not found",
			Error:   wallets.ErrWalletNotFound,
		}
	}
	preview := PreviewPlanResponse{
		CanConfirm:         canConfirm,
		PlanID:             plan.ID,
		PlanAmount:         plan.Amount,
		WalletAvailable:    wallet.AvailableBalance,
		TopLevelAllocated:  topLevelAllocated,
		PlanUnboxedBalance: planUnboxedBalance,
		UnboxedPolicy:      plan.UnboxedBalancePolicy,
		ValidationErrors:   validationErrors,
		EnvelopeTree:       []EnvelopePreviewNode{},
	}

	return &response.APIResponse{
		Success: true,
		Message: "Spending plan preview generated successfully",
		Data:    preview,
	}
}

func (s *PlanService) ConfirmPlan(userID string, planID string, req ConfirmPlanRequest) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	if !req.AcceptedPreview {
		return &response.APIResponse{
			Success: false,
			Message: "You must accept the preview before confirming this plan",
			Error: map[string]string{
				"accepted_preview": "Accepted preview is required",
			},
		}
	}

	var confirmedPlan *SpendingPlan

	err := s.repository.WithTransaction(func(tx *gorm.DB) error {
		plan, err := s.repository.FindByIDAndUserIDForUpdate(tx, planID, userID)
		if err != nil {
			return err
		}

		if plan == nil {
			return ErrPlanNotFound
		}

		if plan.Status != PlanStatusDraft {
			return ErrPlanNotDraft
		}

		if plan.FundingStatus != PlanFundingStatusUnfunded {
			return ErrPlanAlreadyFunded
		}

		if plan.Amount <= 0 {
			return ErrInvalidPlanAmount
		}

		if plan.EndDate.Before(plan.StartDate) || plan.EndDate.Equal(plan.StartDate) {
			return ErrInvalidPlanDateRange
		}

		wallet, balanceBefore, balanceAfter, err := s.walletRepo.LockFundsForSpendingPlanTx(
			tx,
			userID,
			plan.WalletID,
			plan.Amount,
		)
		if err != nil {
			return err
		}

		ledger := &wallets.WalletLedger{
			ID:            security.GenateUniqueID(),
			UserID:        userID,
			WalletID:      plan.WalletID,
			Currency:      wallet.Currency,
			Reference:     "LEDGER_PLAN_LOCK_" + plan.ID,
			Type:          "lock",
			Source:        "spending_plan",
			SourceID:      plan.ID,
			Amount:        plan.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			Description:   "Funds locked for spending plan: " + plan.Name,
		}

		_, err = s.walletRepo.CreateWalletLedgerTx(tx, ledger)
		if err != nil {
			return err
		}

		now := time.Now()

		plan.Status = PlanStatusActive
		plan.FundingStatus = PlanFundingStatusLocked
		plan.ConfirmedAt = &now
		plan.FundedAt = &now

		err = s.repository.UpdateWithTx(tx, plan)
		if err != nil {
			return err
		}

		confirmedPlan = plan

		return nil
	})

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not confirm spending plan",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Spending plan confirmed successfully",
		Data:    mapPlanToResponse(confirmedPlan, 0),
	}
}

func (s *PlanService) CancelPlan(userID string, planID string, req CancelPlanRequest) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	log.Printf("[CancelPlan] started user_id=%s plan_id=%s", userID, planID)

	if planID == "" {
		log.Printf("[CancelPlan] failed: empty plan_id user_id=%s", userID)

		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	reason := strings.TrimSpace(req.Reason)

	if reason == "" {
		log.Printf("[CancelPlan] failed: empty reason user_id=%s plan_id=%s", userID, planID)

		return &response.APIResponse{
			Success: false,
			Message: "Cancellation reason is required",
			Error: map[string]string{
				"reason": "Reason is required",
			},
		}
	}

	var cancelledPlan *SpendingPlan
	var cancelledWallet *wallets.Wallet
	var cancellationFee int64
	var refundedAmount int64

	isDraftCancellation := false
	isFlexibleCancellation := false
	isLockedCancellationBlocked := false

	err := s.repository.WithTransaction(func(tx *gorm.DB) error {
		log.Printf("[CancelPlan] transaction started user_id=%s plan_id=%s", userID, planID)

		plan, err := s.repository.FindByIDAndUserIDForUpdate(tx, planID, userID)
		if err != nil {
			log.Printf("[CancelPlan] failed to fetch plan user_id=%s plan_id=%s error=%v", userID, planID, err)
			return err
		}

		if plan == nil {
			log.Printf("[CancelPlan] plan not found user_id=%s plan_id=%s", userID, planID)
			return ErrPlanNotFound
		}

		log.Printf(
			"[CancelPlan] plan found user_id=%s plan_id=%s status=%s funding_status=%s policy=%s amount=%d wallet_id=%s",
			userID,
			plan.ID,
			plan.Status,
			plan.FundingStatus,
			plan.UnboxedBalancePolicy,
			plan.Amount,
			plan.WalletID,
		)

		if plan.Status == PlanStatusCompleted ||
			plan.Status == PlanStatusCancelled ||
			plan.Status == PlanStatusExpired {
			log.Printf("[CancelPlan] blocked final plan user_id=%s plan_id=%s status=%s", userID, plan.ID, plan.Status)
			return ErrPlanAlreadyFinal
		}

		if plan.Status == PlanStatusDraft {
			log.Printf("[CancelPlan] cancelling draft plan user_id=%s plan_id=%s", userID, plan.ID)

			err = s.repository.MarkAsCancelled(tx, plan, reason)
			if err != nil {
				log.Printf("[CancelPlan] failed to mark draft plan cancelled user_id=%s plan_id=%s error=%v", userID, plan.ID, err)
				return err
			}

			isDraftCancellation = true
			cancelledPlan = plan

			log.Printf("[CancelPlan] draft plan cancelled user_id=%s plan_id=%s", userID, plan.ID)

			return nil
		}

		if plan.Status == PlanStatusActive &&
			plan.UnboxedBalancePolicy == PlanUnboxedPolicyLockedUntilPlanEnds {
			isLockedCancellationBlocked = true

			log.Printf("[CancelPlan] blocked locked plan user_id=%s plan_id=%s policy=%s", userID, plan.ID, plan.UnboxedBalancePolicy)

			return ErrPlanLockedCannotCancel
		}

		if plan.Status == PlanStatusActive &&
			plan.UnboxedBalancePolicy == PlanUnboxedPolicyFlexible {
			log.Printf("[CancelPlan] flexible active plan cancellation started user_id=%s plan_id=%s", userID, plan.ID)

			if plan.FundingStatus != PlanFundingStatusLocked {
				log.Printf(
					"[CancelPlan] blocked unfunded plan user_id=%s plan_id=%s funding_status=%s",
					userID,
					plan.ID,
					plan.FundingStatus,
				)

				return ErrPlanNotFunded
			}

			cancellationFee = calculatePlanCancellationFee(plan.Amount)
			refundedAmount = plan.Amount - cancellationFee
			isFlexibleCancellation = true

			log.Printf(
				"[CancelPlan] fee calculated user_id=%s plan_id=%s amount=%d cancellation_fee=%d refunded_amount=%d",
				userID,
				plan.ID,
				plan.Amount,
				cancellationFee,
				refundedAmount,
			)

			wallet, balanceBefore, balanceAfter, ledgerBefore, ledgerAfter, err := s.walletRepo.CancelFlexiblePlanTx(
				tx,
				userID,
				plan.WalletID,
				plan.Amount,
				cancellationFee,
			)
			if err != nil {
				log.Printf("[CancelPlan] wallet cancellation failed user_id=%s plan_id=%s wallet_id=%s error=%v", userID, plan.ID, plan.WalletID, err)
				return err
			}

			log.Printf(
				"[CancelPlan] wallet updated user_id=%s plan_id=%s wallet_id=%s available_before=%d available_after=%d ledger_before=%d ledger_after=%d locked_balance=%d",
				userID,
				plan.ID,
				wallet.ID,
				balanceBefore,
				balanceAfter,
				ledgerBefore,
				ledgerAfter,
				wallet.LockedBalance,
			)

			unlockLedger := &wallets.WalletLedger{
				ID:            security.GenateUniqueID(),
				UserID:        userID,
				WalletID:      plan.WalletID,
				Currency:      wallet.Currency,
				Reference:     "LEDGER_PLAN_CANCEL_UNLOCK_" + plan.ID,
				Type:          wallets.WalletLedgerTypeUnlock,
				Source:        wallets.WalletLedgerSourceSpendingPlan,
				SourceID:      plan.ID,
				Amount:        refundedAmount,
				BalanceBefore: balanceBefore,
				BalanceAfter:  balanceAfter,
				Description:   "Refunded funds from cancelled flexible spending plan: " + plan.Name,
			}

			_, err = s.walletRepo.CreateWalletLedgerTx(tx, unlockLedger)
			if err != nil {
				log.Printf("[CancelPlan] failed to create unlock ledger user_id=%s plan_id=%s error=%v", userID, plan.ID, err)
				return err
			}

			log.Printf(
				"[CancelPlan] unlock ledger created user_id=%s plan_id=%s ledger_ref=%s amount=%d",
				userID,
				plan.ID,
				unlockLedger.Reference,
				unlockLedger.Amount,
			)

			if cancellationFee > 0 {
				feeLedger := &wallets.WalletLedger{
					ID:            security.GenateUniqueID(),
					UserID:        userID,
					WalletID:      plan.WalletID,
					Currency:      wallet.Currency,
					Reference:     "LEDGER_PLAN_CANCEL_FEE_" + plan.ID,
					Type:          wallets.WalletLedgerTypeDebit,
					Source:        wallets.WalletLedgerSourcePlanCancellationFee,
					SourceID:      plan.ID,
					Amount:        cancellationFee,
					BalanceBefore: ledgerBefore,
					BalanceAfter:  ledgerAfter,
					Description:   "Early cancellation fee for flexible spending plan: " + plan.Name,
				}

				_, err = s.walletRepo.CreateWalletLedgerTx(tx, feeLedger)
				if err != nil {
					log.Printf("[CancelPlan] failed to create fee ledger user_id=%s plan_id=%s error=%v", userID, plan.ID, err)
					return err
				}

				log.Printf(
					"[CancelPlan] fee ledger created user_id=%s plan_id=%s ledger_ref=%s amount=%d",
					userID,
					plan.ID,
					feeLedger.Reference,
					feeLedger.Amount,
				)
			}

			now := time.Now()

			plan.Status = PlanStatusCancelled
			plan.FundingStatus = PlanFundingStatusReleased
			plan.CancelledAt = &now
			plan.CancelReason = reason

			err = s.repository.UpdateWithTx(tx, plan)
			if err != nil {
				log.Printf("[CancelPlan] failed to update plan status user_id=%s plan_id=%s error=%v", userID, plan.ID, err)
				return err
			}

			cancelledPlan = plan
			cancelledWallet = wallet

			log.Printf("[CancelPlan] flexible plan cancelled user_id=%s plan_id=%s", userID, plan.ID)

			return nil
		}

		log.Printf(
			"[CancelPlan] unsupported cancellation state user_id=%s plan_id=%s status=%s policy=%s funding_status=%s",
			userID,
			plan.ID,
			plan.Status,
			plan.UnboxedBalancePolicy,
			plan.FundingStatus,
		)

		return ErrPlanCannotCancel
	})

	if err != nil {
		log.Printf(
			"[CancelPlan] failed user_id=%s plan_id=%s is_draft=%t is_flexible=%t is_locked_blocked=%t error=%v",
			userID,
			planID,
			isDraftCancellation,
			isFlexibleCancellation,
			isLockedCancellationBlocked,
			err,
		)

		if err == ErrPlanNotFound {
			return &response.APIResponse{
				Success: false,
				Message: "Spending plan not found",
				Error:   err,
			}
		}

		if err == ErrPlanAlreadyFinal {
			return &response.APIResponse{
				Success: false,
				Message: "This spending plan cannot be cancelled",
				Error:   err,
			}
		}

		if err == ErrPlanLockedCannotCancel {
			return &response.APIResponse{
				Success: false,
				Message: "This plan is locked and cannot be cancelled early",
				Error:   err,
			}
		}

		if err == ErrPlanNotFunded {
			return &response.APIResponse{
				Success: false,
				Message: "This plan has not been funded",
				Error:   err,
			}
		}

		return &response.APIResponse{
			Success: false,
			Message: "Could not cancel spending plan",
			Error:   err,
		}
	}

	var walletSnapshot *WalletSnapshotDTO

	if cancelledWallet != nil {
		walletSnapshot = &WalletSnapshotDTO{
			WalletID:         cancelledWallet.ID,
			AvailableBalance: cancelledWallet.AvailableBalance,
			LockedBalance:    cancelledWallet.LockedBalance,
			LedgerBalance:    cancelledWallet.LedgerBalance,
			Currency:         cancelledWallet.Currency,
		}
	}

	log.Printf(
		"[CancelPlan] successful user_id=%s plan_id=%s is_draft=%t is_flexible=%t cancellation_fee=%d refunded_amount=%d",
		userID,
		planID,
		isDraftCancellation,
		isFlexibleCancellation,
		cancellationFee,
		refundedAmount,
	)

	return &response.APIResponse{
		Success: true,
		Message: "Spending plan cancelled successfully",
		Data: CancelPlanResponse{
			Plan:            mapPlanToResponse(cancelledPlan, 0),
			Wallet:          walletSnapshot,
			Message:         "Plan cancelled successfully",
			CancellationFee: cancellationFee,
			RefundedAmount:  refundedAmount,
		},
	}
}

func calculatePlanCancellationFee(amount int64) int64 {
	fee := (amount * PlanCancellationFeePercent) / 100

	if fee > PlanCancellationFeeCap {
		return PlanCancellationFeeCap
	}

	return fee
}
