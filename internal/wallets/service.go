package wallets

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"

	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
)

type WalletService struct {
	repo               *WalletRepository
	payment            paymentproviders.PaymentProvider
	idempotencyManager *client.IdempotentRequest
}

func NewWalletService(repo *WalletRepository, payment paymentproviders.PaymentProvider, idempotencyManager *client.IdempotentRequest) *WalletService {

	return &WalletService{repo: repo, payment: payment, idempotencyManager: idempotencyManager}
}

func (svc *WalletService) GetMyWallet(userId string) *response.APIResponse {
	wallet, err := svc.repo.GetWalletByOwnerID(userId)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: false,
		Message: "Invalid data",
		Error:   wallet,
	}
}

func (s *WalletService) CreateWallet(userId string) *response.APIResponse {
	// Check If user has wallet

	wallet, _ := s.repo.GetWalletByOwnerID(userId)
	if wallet != nil {
		return &response.APIResponse{
			Success: false,
			Message: "User already has a wallet",
			Error:   "wallet_exists",
		}
	}
	now := time.Now()
	id := security.GenateUniqueID()
	walletReq := &Wallet{
		ID:               id,
		UserID:           userId,
		LockedBalance:    0,
		AvailableBalance: 0,
		LedgerBalance:    0,
		Currency:         "NGN",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	wallet, err := s.repo.Create(walletReq)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not create wallet",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet created successfully",
		Data:    wallet,
	}

}

func (s *WalletService) InitiateDeposit(
	ctx context.Context,
	idempotencyKey string,
	userId string,
	email string,
	req InitiateDepositRequest,
) *response.APIResponse {
	// The reference is tied safely to this specific idempotent execution run

	now := time.Now()
	id := security.GenateUniqueID()
	reference := security.GenerateReference()
	wallet, err := s.repo.GetWalletByOwnerID(userId)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   err,
		}
	}
	// 2. Define the core business logic wrapper
	executionLogic := func() (int, interface{}, error) {
		APP_ENV := os.Getenv("APP_ENV")
		callback_url := fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_LOCAL_URL"))

		if APP_ENV == "production" {
			callback_url = fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_URL"))
		}

		reqBody := &paymentproviders.DepositBody{
			Amount:      req.Amount,
			Currency:    "NGN",
			Email:       email,
			Reference:   reference,
			CallbackURL: callback_url,
		}

		paystackResponse, err := s.payment.InitiateDeposit(ctx, reqBody)
		if err != nil {
			// Return appropriate HTTP error status alongside the raw error object
			return http.StatusBadRequest, nil, err
		}

		// Return HTTP Status 200 along with Paystack's initialization payload data
		return http.StatusOK, paystackResponse.Data, nil
	}

	// 3. Funnel execution through the idempotency machine
	_, payload, err := s.idempotencyManager.HandleIdempotentRequest(ctx, idempotencyKey, userId, executionLogic)
	if err != nil {
		// If there's an internal error or a conflicting duplicate key request
		return &response.APIResponse{
			Success: false,
			Message: "Failed to process deposit request",
			Error:   err.Error(),
		}
	}
	body := &WalletDeposit{
		ID:        id,
		Amount:    req.Amount,
		Currency:  "NGN",
		Reference: reference,
		WalletID:  wallet.ID,
		UserID:    userId,
		Status:    "pending",
		Provider:  "paystack",
		CreatedAt: now,
		UpdatedAt: now,
	}
	// create deposit log
	_, err = s.repo.CreateDeposit(body)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to create deposit log",
			Error:   err,
		}
	}
	return &response.APIResponse{
		Success: true,
		Message: "Deposit initiated successfully",
		Data:    payload,
	}
}
