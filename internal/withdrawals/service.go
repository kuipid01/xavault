package withdrawals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/funds"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/pricing"
	httpclient "github.com/kuipid01/salary-vault/internal/shared/client"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type WithdrawalService struct {
	repo               *WithdrawalRepository
	planRepo           *spendingplans.PlanRepository
	envelopeRepo       *envelope.EnvelopeRepo
	walletRepo         *wallets.WalletRepository
	bankAccountRepo    *bankaccounts.Repository
	userRepo           *users.UserRepository
	payment            paymentproviders.PaymentProvider
	idempotencyManager *client.IdempotentRequest
	fundsRepo          *funds.Repository
	pricing            *pricing.Service
	subRepo            *subscription.SubscriptionRepository
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
	fundsRepo *funds.Repository,
	extras ...interface{},
) *WithdrawalService {
	pricingService := pricing.NewService(repo.db)
	subscriptionRepo := subscription.NewSubscriptionRepository(repo.db)
	for _, extra := range extras {
		switch value := extra.(type) {
		case *pricing.Service:
			pricingService = value
		case *subscription.SubscriptionRepository:
			subscriptionRepo = value
		}
	}
	return &WithdrawalService{
		repo:               repo,
		planRepo:           planRepo,
		envelopeRepo:       envelopeRepo,
		walletRepo:         walletRepo,
		bankAccountRepo:    bankAccountRepo,
		userRepo:           userRepo,
		payment:            payment,
		idempotencyManager: idempotencyManager,
		fundsRepo:          fundsRepo,
		pricing:            pricingService,
		subRepo:            subscriptionRepo,
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

	if !isPlanWithdrawableStatus(plan.Status) {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan is not active",
			Error:   ErrPlanNotActive,
		}
	}

	preview, err := s.calculateWithdrawalPreview(userID, planID, req.SourceType, req.EnvelopeID, req.Amount)

	if err != nil {
		return withdrawalErrorResponse("Could not preview withdrawal", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return withdrawalErrorResponse("Could not quote withdrawal", err)
	}
	allowance, err := s.subRepo.HasFreeWithdrawal(userUUID)
	if err != nil {
		return withdrawalErrorResponse("Could not quote withdrawal", err)
	}
	contextBytes, _ := json.Marshal(map[string]interface{}{"plan_id": planID, "source_type": req.SourceType, "envelope_id": req.EnvelopeID})
	quote, err := s.pricing.CreateWithdrawalQuote(userID, req.Amount, allowance, string(contextBytes))
	if err != nil {
		return withdrawalErrorResponse("Could not quote withdrawal", err)
	}
	preview.QuoteID = quote.ID
	preview.ProviderTransferFee = quote.ProviderFee
	preview.GovernmentLevy = quote.GovernmentLevy
	preview.XavaultSubsidy = quote.XavaultSubsidy
	preview.UserFee = quote.UserFee
	preview.TotalDeduction = quote.TotalDeduction
	preview.RecipientAmount = quote.RecipientAmount
	preview.AllowanceApplied = quote.AllowanceApplied
	preview.PricingVersion = quote.PricingVersion
	preview.ExpiresAt = quote.ExpiresAt.Format(time.RFC3339)
	if quote.TotalDeduction > preview.WithdrawableBalance {
		preview.CanWithdraw = false
		preview.ValidationErrors = append(preview.ValidationErrors, "Insufficient balance for withdrawal and provider charges")
	}
	preview.RemainingBalance = preview.WithdrawableBalance - quote.TotalDeduction
	if preview.RemainingBalance < 0 {
		preview.RemainingBalance = 0
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
	req.QuoteID = strings.TrimSpace(req.QuoteID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}
	if req.QuoteID == "" {
		return &response.APIResponse{Success: false, Message: "Withdrawal quote is required", Error: "quote_id_required"}
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
	requestHash := hashWithdrawalRequest(planID, req, recipientCode)

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
			var feeQuote *pricing.Quote

			now := time.Now()

			withdrawalID := security.GenateUniqueID()
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

				planCurrency = plan.Currency
				feeQuote, err = s.pricing.ConsumeTx(tx, req.QuoteID, userID, pricing.OperationWithdrawal)
				if err != nil {
					return err
				}
				if feeQuote.PrincipalAmount != req.Amount {
					return pricing.ErrQuoteInvalid
				}
				var quoteContext struct {
					PlanID     string  `json:"plan_id"`
					SourceType string  `json:"source_type"`
					EnvelopeID *string `json:"envelope_id"`
				}
				if err := json.Unmarshal([]byte(feeQuote.ContextJSON), &quoteContext); err != nil ||
					quoteContext.PlanID != planID ||
					quoteContext.SourceType != req.SourceType ||
					!sameOptionalString(quoteContext.EnvelopeID, req.EnvelopeID) {
					return pricing.ErrQuoteInvalid
				}
				allowancePeriod := ""
				if feeQuote.AllowanceApplied {
					reserved, period, reserveErr := s.subRepo.ReserveFreeWithdrawalTx(tx, uuid.MustParse(userID))
					if reserveErr != nil {
						return reserveErr
					}
					if !reserved {
						return pricing.ErrQuoteInvalid
					}
					allowancePeriod = period
				}

				err = s.ensureNoBlockingOpenWithdrawalTx(
					tx,
					userID,
					planID,
					req.SourceType,
					req.EnvelopeID,
				)
				if err != nil {
					return err
				}

				// Calculate after stale pending requests are cleared so reservations are current.
				preview, err := s.calculateWithdrawalPreviewTx(tx, userID, planID, req.SourceType, req.EnvelopeID, feeQuote.TotalDeduction)
				if err != nil {
					return err
				}
				if !preview.CanWithdraw {
					return ErrInsufficientWithdrawable
				}

				// Critical money check:
				// Confirmed spending plan funds already live in locked_balance.
				// Keep balances unchanged until Paystack confirms transfer success.
				updatedWallet, err = s.walletRepo.VerifyLockedFundsForWithdrawalTx(
					tx,
					userID,
					plan.WalletID,
					feeQuote.TotalDeduction,
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
					QuoteID:             &feeQuote.ID,
					ProviderTransferFee: feeQuote.ProviderFee,
					GovernmentLevy:      feeQuote.GovernmentLevy,
					XavaultSubsidy:      feeQuote.XavaultSubsidy,
					UserFee:             feeQuote.UserFee,
					TotalDeduction:      feeQuote.TotalDeduction,
					RecipientAmount:     feeQuote.RecipientAmount,
					AllowanceApplied:    feeQuote.AllowanceApplied,
					AllowanceState:      map[bool]string{true: "reserved", false: "none"}[feeQuote.AllowanceApplied],
					PricingVersion:      feeQuote.PricingVersion,
				}
				if feeQuote.AllowanceApplied {
					withdrawal.AllowancePeriod = allowancePeriod
				}

				if err := s.repo.CreateWithTx(tx, withdrawal); err != nil {
					return err
				}

				createdWithdrawal = withdrawal

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
			amountForPaystack := feeQuote.RecipientAmount

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
				// The provider may have accepted the transfer before the network
				// error reached us. Keep the original reference pending so
				// reconciliation can verify it and new keys cannot create another.
				return http.StatusAccepted, map[string]interface{}{
					"message":    "Withdrawal request is pending provider confirmation",
					"withdrawal": mapWithdrawalToResponse(createdWithdrawal),
					"wallet": WalletSnapshotDTO{
						WalletID:         updatedWallet.ID,
						AvailableBalance: updatedWallet.AvailableBalance,
						LockedBalance:    updatedWallet.LockedBalance,
						LedgerBalance:    updatedWallet.LedgerBalance,
						Currency:         updatedWallet.Currency,
					},
					"note": "Provider confirmation will be resolved by webhook or reconciliation",
				}, nil
			}
			if strings.TrimSpace(paystackResp.Data.TransferCode) == "" ||
				paystackResp.Data.Amount != amountForPaystack ||
				!strings.EqualFold(paystackResp.Data.Currency, planCurrency) {
				return http.StatusInternalServerError, nil, errors.New("provider returned incomplete or mismatched transfer details")
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
		requestHash,
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

// CancelWithdrawal cancels a withdrawal the user no longer wants. Only a
// pending withdrawal whose transfer never reached the provider can be
// cancelled: no money has moved locally (plan funds stay locked until
// Paystack confirms), so cancelling just releases the reservation and stops
// all polling for it. Withdrawals already sent to Paystack cannot be undone
// and return ErrWithdrawalAlreadyProcessing.
func (s *WithdrawalService) CancelWithdrawal(userID string, withdrawalID string, reason string) *response.APIResponse {
	withdrawalID = strings.TrimSpace(withdrawalID)
	if withdrawalID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal ID is required",
			Error:   "withdrawal_id_required",
		}
	}

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		withdrawal, err := s.repo.FindByIDForUpdate(tx, withdrawalID)
		if err != nil {
			return err
		}
		if withdrawal == nil || withdrawal.UserID != userID {
			return ErrWithdrawalNotFound
		}
		if withdrawal.Status != WithdrawalStatusPending {
			return ErrWithdrawalNotCancellable
		}
		if strings.TrimSpace(withdrawal.ProviderReference) != "" {
			return ErrWithdrawalAlreadyProcessing
		}
		if withdrawal.AllowanceApplied && withdrawal.AllowanceState == "reserved" {
			if err := s.subRepo.ReleaseFreeWithdrawalTx(tx, uuid.MustParse(withdrawal.UserID), withdrawal.AllowancePeriod, false); err != nil {
				return err
			}
			if err := tx.Model(withdrawal).Update("allowance_state", "released").Error; err != nil {
				return err
			}
		}
		return s.repo.MarkCancelledWithTx(tx, withdrawal.ID, reason)
	})
	if err != nil {
		return withdrawalErrorResponse("Could not cancel withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal cancelled successfully",
	}
}

// FailStaleWithdrawal terminates a withdrawal that never resolved within the
// reconciliation window so it stops being polled against the provider on every
// run. Safe because no money is debited locally until Paystack confirms
// success; a late provider success is still honored by
// CompleteProviderWithdrawalTx.
func (s *WithdrawalService) FailStaleWithdrawal(reference string, reason string) *response.APIResponse {
	return s.FailProviderWithdrawal(reference, "failed", reason)
}

func (s *WithdrawalService) markFailedWithdrawalInitiation(
	withdrawalID string,
	reason string,
) error {
	return s.repo.WithTransaction(func(tx *gorm.DB) error {
		withdrawal, err := s.repo.FindByIDForUpdate(tx, withdrawalID)
		if err != nil {
			return err
		}
		if withdrawal == nil {
			return gorm.ErrRecordNotFound
		}
		if withdrawal.Status != WithdrawalStatusPending {
			return nil
		}
		if withdrawal.AllowanceApplied && withdrawal.AllowanceState == "reserved" {
			if err := s.subRepo.ReleaseFreeWithdrawalTx(tx, uuid.MustParse(withdrawal.UserID), withdrawal.AllowancePeriod, false); err != nil {
				return err
			}
			if err := tx.Model(&withdrawal).Update("allowance_state", "released").Error; err != nil {
				return err
			}
		}
		return s.repo.MarkFailedWithTx(tx, withdrawalID, reason)
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

func (s *WithdrawalService) GetUserWithdrawals(userID string, page int, limit int) *response.APIResponse {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	withdrawals, total, err := s.repo.FindByUserID(userID, limit, (page-1)*limit)
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
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}
}

func (s *WithdrawalService) ensureNoBlockingOpenWithdrawalTx(
	tx *gorm.DB,
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
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

	// A pending record may represent a transfer Paystack accepted before the
	// provider response could be persisted. Never retry that uncertainty as a
	// new transfer; webhook processing or reconciliation must resolve it.
	return ErrWithdrawalRequestInProgress
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
	return s.completeProviderWithdrawal(reference, providerStatus, 0, "", "", false)
}

func (s *WithdrawalService) CompleteProviderWithdrawalWithDetails(reference string, providerStatus string, providerAmount int64, providerCurrency, providerTransferCode string) *response.APIResponse {
	return s.completeProviderWithdrawal(reference, providerStatus, providerAmount, providerCurrency, providerTransferCode, true)
}

func (s *WithdrawalService) completeProviderWithdrawal(reference string, providerStatus string, providerAmount int64, providerCurrency, providerTransferCode string, validateProviderDetails bool) *response.APIResponse {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawal reference is required",
			Error:   "withdrawal_reference_required",
		}
	}

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		return s.completeProviderWithdrawalTx(tx, reference, providerStatus, providerAmount, providerCurrency, providerTransferCode, validateProviderDetails)
	})

	if err != nil {
		return withdrawalErrorResponse("Could not complete withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal completed successfully",
	}
}

func (s *WithdrawalService) CompleteProviderWithdrawalTx(tx *gorm.DB, reference, providerStatus string) error {
	return s.completeProviderWithdrawalTx(tx, reference, providerStatus, 0, "", "", false)
}

func (s *WithdrawalService) CompleteProviderWithdrawalTxWithDetails(tx *gorm.DB, reference, providerStatus string, providerAmount int64, providerCurrency, providerTransferCode string) error {
	return s.completeProviderWithdrawalTx(tx, reference, providerStatus, providerAmount, providerCurrency, providerTransferCode, true)
}

func (s *WithdrawalService) completeProviderWithdrawalTx(tx *gorm.DB, reference, providerStatus string, providerAmount int64, providerCurrency, providerTransferCode string, validateProviderDetails bool) error {
	withdrawal, err := s.repo.FindByReferenceForUpdate(tx, strings.TrimSpace(reference))
	if err != nil {
		return err
	}
	if withdrawal == nil {
		return ErrWithdrawalNotFound
	}
	if withdrawal.Status == WithdrawalStatusPaid {
		return nil
	}
	// Bug fix: a failed or cancelled withdrawal never had money debited
	// locally (the debit happens only in this function), so a late provider
	// success must still debit and mark it paid - otherwise the user would
	// keep the plan funds AND receive the bank transfer. Reversed stays
	// terminal: it was already refunded.
	if withdrawal.Status == WithdrawalStatusReversed {
		return fmt.Errorf("withdrawal is already final with status %s", withdrawal.Status)
	}
	if validateProviderDetails && providerAmount <= 0 {
		return errors.New("provider amount is missing")
	}
	if validateProviderDetails && providerAmount != withdrawal.RecipientAmount {
		return fmt.Errorf("provider amount %d does not match withdrawal amount %d", providerAmount, withdrawal.RecipientAmount)
	}
	if validateProviderDetails && strings.TrimSpace(providerCurrency) == "" {
		return errors.New("provider currency is missing")
	}
	if validateProviderDetails && !strings.EqualFold(providerCurrency, withdrawal.Currency) {
		return fmt.Errorf("provider currency %q does not match withdrawal currency %q", providerCurrency, withdrawal.Currency)
	}
	if validateProviderDetails && strings.TrimSpace(providerTransferCode) == "" {
		return errors.New("provider transfer code is missing")
	}
	if validateProviderDetails && withdrawal.ProviderTransferCode != "" && providerTransferCode != withdrawal.ProviderTransferCode {
		return fmt.Errorf("provider transfer code does not match withdrawal")
	}
	totalDeduction := withdrawal.TotalDeduction
	if totalDeduction <= 0 {
		totalDeduction = withdrawal.Amount
	}
	ledgerBefore, updatedWallet, err := s.walletRepo.CompleteWithdrawalTx(
		tx, withdrawal.UserID, withdrawal.WalletID, totalDeduction,
	)
	if err != nil {
		return err
	}
	if err := s.repo.MarkPaidWithTx(tx, withdrawal.ID, providerStatus); err != nil {
		return err
	}
	if withdrawal.AllowanceApplied && withdrawal.AllowanceState == "reserved" {
		if err := s.subRepo.FinalizeFreeWithdrawalTx(tx, uuid.MustParse(withdrawal.UserID), withdrawal.AllowancePeriod); err != nil {
			return err
		}
		if err := tx.Model(withdrawal).Update("allowance_state", "used").Error; err != nil {
			return err
		}
	}
	_, err = s.walletRepo.CreateWalletLedgerTx(tx, &wallets.WalletLedger{
		ID: security.GenateUniqueID(), UserID: withdrawal.UserID, WalletID: withdrawal.WalletID,
		Currency: withdrawal.Currency, Reference: withdrawal.Reference,
		Source: wallets.WalletLedgerSourceWithdrawal, SourceID: withdrawal.ID,
		Type: wallets.WalletLedgerTypeWithdrawalDebit, Amount: totalDeduction,
		BalanceBefore: ledgerBefore, BalanceAfter: updatedWallet.LedgerBalance,
		Description: "Withdrawal completed by Paystack", CreatedAt: time.Now(),
	})
	return err
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
		return s.FailProviderWithdrawalTx(tx, reference, providerStatus, reason)
	})

	if err != nil {
		return withdrawalErrorResponse("Could not fail withdrawal", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal marked failed successfully",
	}
}

func (s *WithdrawalService) FailProviderWithdrawalTx(tx *gorm.DB, reference, providerStatus, reason string) error {
	withdrawal, err := s.repo.FindByReferenceForUpdate(tx, strings.TrimSpace(reference))
	if err != nil {
		return err
	}
	if withdrawal == nil {
		return ErrWithdrawalNotFound
	}
	if withdrawal.Status == WithdrawalStatusReversed ||
		withdrawal.Status == WithdrawalStatusFailed ||
		withdrawal.Status == WithdrawalStatusCancelled {
		return nil
	}
	if withdrawal.Status == WithdrawalStatusPaid {
		totalDeduction := withdrawal.TotalDeduction
		if totalDeduction <= 0 {
			totalDeduction = withdrawal.Amount
		}
		ledgerBefore, wallet, err := s.walletRepo.ReverseCompletedWithdrawalTx(
			tx, withdrawal.UserID, withdrawal.WalletID, totalDeduction,
		)
		if err != nil {
			return err
		}
		if err := s.repo.MarkReversedWithTx(tx, withdrawal.ID, providerStatus, reason); err != nil {
			return err
		}
		if withdrawal.AllowanceApplied && withdrawal.AllowanceState == "used" {
			if err := s.subRepo.ReleaseFreeWithdrawalTx(tx, uuid.MustParse(withdrawal.UserID), withdrawal.AllowancePeriod, true); err != nil {
				return err
			}
			if err := tx.Model(withdrawal).Update("allowance_state", "restored").Error; err != nil {
				return err
			}
		}
		_, err = s.walletRepo.CreateWalletLedgerTx(tx, &wallets.WalletLedger{
			ID: security.GenateUniqueID(), UserID: withdrawal.UserID, WalletID: withdrawal.WalletID,
			Currency: withdrawal.Currency, Reference: "WITHDRAWAL_REVERSED_" + withdrawal.ID,
			Source: wallets.WalletLedgerSourceWithdrawal, SourceID: withdrawal.ID,
			Type: wallets.WalletLedgerTypeRefund, Amount: totalDeduction,
			BalanceBefore: ledgerBefore, BalanceAfter: wallet.LedgerBalance,
			Description: "Paystack reversed withdrawal", CreatedAt: time.Now(),
		})
		return err
	}
	if reason == "" {
		reason = "Paystack transfer " + providerStatus
	}
	if withdrawal.AllowanceApplied && withdrawal.AllowanceState == "reserved" {
		if err := s.subRepo.ReleaseFreeWithdrawalTx(tx, uuid.MustParse(withdrawal.UserID), withdrawal.AllowancePeriod, false); err != nil {
			return err
		}
		if err := tx.Model(withdrawal).Update("allowance_state", "released").Error; err != nil {
			return err
		}
	}
	return s.repo.MarkFailedWithTx(tx, withdrawal.ID, reason)
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
		if plan.UnboxedBalancePolicy == spendingplans.PlanUnboxedPolicyLockedUntilPlanEnds && time.Now().Before(plan.EndDate) {
			return nil, ErrSpendingPlanLockedUntilPlanEnds
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
		// The plan is the security boundary. An envelope cannot provide an
		// early-withdrawal path around a locked parent plan, including for
		// legacy rows whose stored envelope policy is flexible.
		if plan.UnboxedBalancePolicy == spendingplans.PlanUnboxedPolicyLockedUntilPlanEnds && time.Now().Before(plan.EndDate) {
			return nil, ErrSpendingPlanLockedUntilPlanEnds
		}

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
	if remainingBalance < 0 {
		remainingBalance = 0
	}

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

	withdrawalTotals, err := s.fundsRepo.PlanUnboxedTotals(tx, planID, userID)
	if err != nil {
		return 0, err
	}

	withdrawable := funds.AvailableBalance(planAmount, topLevelAllocated, withdrawalTotals)

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
	if env.UnboxedBalancePolicy == envelope.EnvelopeUnboxedPolicyLockedUntilParentEnds && time.Now().Before(env.EndDate) {
		return 0, ErrEnvelopeLockedUntilEnd
	}

	childAllocated, err := s.envelopeRepo.SumChildEnvelopeAmountTx(tx, env.ID)
	if err != nil {
		return 0, err
	}

	withdrawalTotals, err := s.fundsRepo.EnvelopeTotals(tx, env.ID, userID)
	if err != nil {
		return 0, err
	}

	withdrawable := funds.AvailableBalance(env.Amount, childAllocated, withdrawalTotals)

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
	err := s.userRepo.VerifyWithdrawalPin(userID, pin)
	switch {
	case errors.Is(err, users.ErrWithdrawalPinNotSet):
		return ErrWithdrawalPinNotSet
	case errors.Is(err, users.ErrWithdrawalPinInvalid):
		return ErrInvalidWithdrawalPin
	case errors.Is(err, users.ErrWithdrawalPinLocked):
		return ErrWithdrawalPinLocked
	default:
		return err
	}
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

func sameOptionalString(left, right *string) bool {
	leftValue := ""
	rightValue := ""
	if left != nil {
		leftValue = strings.TrimSpace(*left)
	}
	if right != nil {
		rightValue = strings.TrimSpace(*right)
	}
	return leftValue == rightValue
}

func hashWithdrawalRequest(planID string, req RequestWithdrawalRequest, recipientCode string) string {
	payload, _ := json.Marshal(struct {
		PlanID          string  `json:"plan_id"`
		QuoteID         string  `json:"quote_id"`
		SourceType      string  `json:"source_type"`
		EnvelopeID      *string `json:"envelope_id"`
		Amount          int64   `json:"amount"`
		Reason          string  `json:"reason"`
		DestinationType string  `json:"destination_type"`
		DestinationID   string  `json:"destination_id"`
		RecipientCode   string  `json:"recipient_code"`
	}{planID, req.QuoteID, req.SourceType, req.EnvelopeID, req.Amount, req.Reason, req.DestinationType, req.DestinationID, recipientCode})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
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

	if err == ErrSpendingPlanLockedUntilPlanEnds || err == ErrEnvelopeLockedUntilEnd {
		return &response.APIResponse{Success: false, Message: err.Error(), Error: err}
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
			Message: "Incorrect withdrawal PIN. One attempt remaining before withdrawals are locked.",
			Error:   err,
		}
	}

	if err == ErrWithdrawalPinLocked {
		return &response.APIResponse{
			Success: false,
			Message: "Withdrawals are locked after two incorrect PIN attempts. Reset your PIN to continue.",
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
