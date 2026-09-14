package wallets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/pricing"

	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"gorm.io/gorm"
)

type WalletService struct {
	repo               *WalletRepository
	payment            paymentproviders.PaymentProvider
	idempotencyManager *client.IdempotentRequest
	subRepo            *subscription.SubscriptionRepository
	pricing            *pricing.Service
}

func NewWalletService(
	repo *WalletRepository,
	payment paymentproviders.PaymentProvider,
	idempotencyManager *client.IdempotentRequest,
	subRepo *subscription.SubscriptionRepository,
	pricingServices ...*pricing.Service,
) *WalletService {
	pricingService := pricing.NewService(repo.db)
	if len(pricingServices) > 0 && pricingServices[0] != nil {
		pricingService = pricingServices[0]
	}

	return &WalletService{
		repo:               repo,
		payment:            payment,
		idempotencyManager: idempotencyManager,
		subRepo:            subRepo,
		pricing:            pricingService,
	}
}

func (s *WalletService) QuoteDeposit(userID string, req DepositQuoteRequest) *response.APIResponse {
	if req.Amount <= 0 {
		return &response.APIResponse{Success: false, Message: "Deposit amount must be greater than zero", Error: "invalid_amount"}
	}
	if err := s.canAcceptDeposit(nil, userID, req.Amount); err != nil {
		return depositLimitResponse(err)
	}
	quote, err := s.pricing.CreateDepositQuote(userID, req.Amount)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not calculate deposit fee", Error: err.Error()}
	}
	return &response.APIResponse{Success: true, Message: "Deposit fee quoted successfully", Data: quote}
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
		Success: true,
		Message: "Wallet fetched successfully",
		Data:    wallet,
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

func (s *WalletService) EnsureDedicatedAccount(ctx context.Context, userID string, email string) *response.APIResponse {
	wallet, err := s.repo.GetWalletByOwnerID(userID)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch wallet",
			Error:   err.Error(),
		}
	}

	if wallet.DedicatedAccountNumber != "" {
		return &response.APIResponse{
			Success: true,
			Message: "Dedicated account fetched successfully",
			Data:    wallet,
		}
	}

	customerCode := wallet.PaystackCustomerCode
	if customerCode == "" {
		firstName, lastName := namesFromEmail(email)
		customer, err := s.payment.CreateCustomer(ctx, &paymentproviders.CreateCustomerBody{
			Email:     email,
			FirstName: firstName,
			LastName:  lastName,
			Phone:     wallet.User.PhoneNumber,
		})
		if err != nil {
			return &response.APIResponse{
				Success: false,
				Message: "Could not create Paystack customer",
				Error:   err.Error(),
			}
		}

		customerCode = customer.Data.CustomerCode
		wallet.PaystackCustomerCode = customerCode
	}

	preferredBank := strings.TrimSpace(os.Getenv("PAYSTACK_DEDICATED_ACCOUNT_BANK"))
	if preferredBank == "" && strings.HasPrefix(os.Getenv("PAYSTACK_SECRET_KEY"), "sk_test") {
		preferredBank = "test-bank"
	}

	dedicated, err := s.payment.CreateDedicatedAccount(ctx, &paymentproviders.CreateDedicatedAccountBody{
		Customer:      customerCode,
		PreferredBank: preferredBank,
	})
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not create transfer account",
			Error:   err.Error(),
		}
	}

	wallet.DedicatedAccountID = fmt.Sprintf("%d", dedicated.Data.ID)
	wallet.DedicatedAccountNumber = dedicated.Data.AccountNumber
	wallet.DedicatedAccountName = dedicated.Data.AccountName
	wallet.DedicatedBankName = dedicated.Data.Bank.Name
	wallet.DedicatedBankSlug = dedicated.Data.Bank.Slug

	updatedWallet, err := s.repo.Update(wallet)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save transfer account",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Dedicated account created successfully",
		Data:    updatedWallet,
	}
}

func (s *WalletService) InitiateDeposit(
	ctx context.Context,
	idempotencyKey string,
	userId string,
	email string,
	req InitiateDepositRequest,
) *response.APIResponse {
	now := time.Now()
	id := security.GenateUniqueID()
	reference := security.GenerateReference()

	// 1. Move ALL stateful execution inside this function block
	executionLogic := func() (int, interface{}, error) {
		quote, err := s.pricing.Consume(strings.TrimSpace(req.QuoteID), userId, pricing.OperationDeposit)
		if err != nil {
			return http.StatusBadRequest, nil, fmt.Errorf("deposit quote expired; request a new quote: %w", err)
		}
		if req.Amount != quote.PrincipalAmount {
			return http.StatusBadRequest, nil, errors.New("deposit amount does not match quote")
		}

		wallet, err := s.repo.GetWalletByOwnerID(userId)
		if err != nil {
			return http.StatusBadRequest, nil, fmt.Errorf("invalid wallet data: %w", err)
		}
		if err := s.canAcceptDeposit(nil, userId, req.Amount); err != nil {
			return http.StatusBadRequest, nil, err
		}

		APP_ENV := os.Getenv("APP_ENV")
		callback_url := fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_LOCAL_URL"))

		if APP_ENV == "production" {
			callback_url = fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_URL"))
		}

		// cancel_action lets the in-app checkout WebView intercept the
		// customer pressing "cancel" on the Paystack page (window.close()
		// does not work in a WebView). It mirrors the callback URL with a
		// marker the app recognises as a cancellation rather than a payment.
		metadata := map[string]interface{}{
			"cancel_action": callback_url + "?status=cancelled",
		}

		reqBody := &paymentproviders.DepositBody{
			Amount:      quote.GrossCharge,
			Currency:    "NGN",
			Email:       email,
			Reference:   reference,
			CallbackURL: callback_url,
			Metadata:    metadata,
		}

		// Hit Paystack
		paystackResponse, err := s.payment.InitiateDeposit(ctx, reqBody)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		// Create the local deposit log record here
		body := &WalletDeposit{
			ID:                 id,
			Amount:             quote.GrossCharge,
			Currency:           "NGN",
			Reference:          reference,
			WalletID:           wallet.ID,
			UserID:             userId,
			Status:             "pending",
			Provider:           "paystack",
			QuoteID:            &quote.ID,
			GrossAmount:        quote.GrossCharge,
			WalletCreditAmount: quote.NetCredit,
			ProviderFee:        quote.ProviderFee,
			PricingVersion:     quote.PricingVersion,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		_, err = s.repo.CreateDeposit(body)
		if err != nil {
			// If DB logging fails, return an Internal Server Error status
			return http.StatusInternalServerError, nil, fmt.Errorf("failed to create deposit log: %w", err)
		}

		paystackData := paystackPayloadData(paystackResponse.Data)
		payload := InitiateDepositResponse{
			AuthorizationURL: stringFromMap(paystackData, "authorization_url"),
			AccessCode:       stringFromMap(paystackData, "access_code"),
			Reference:        reference,
			ProviderResponse: paystackResponse.Data,
		}

		if providerReference := stringFromMap(paystackData, "reference"); providerReference != "" {
			payload.Reference = providerReference
		}

		return http.StatusOK, payload, nil
	}

	// 2. Funnel execution through the idempotency machine
	_, payload, err := s.idempotencyManager.HandleIdempotentRequest(ctx, idempotencyKey, userId, executionLogic)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to process deposit request",
			Error:   err.Error(),
		}
	}

	// 3. Clean and clear return path for both new and cached runs
	return &response.APIResponse{
		Success: true,
		Message: "Deposit initiated successfully",
		Data:    payload,
	}
}

func namesFromEmail(email string) (string, string) {
	local := strings.TrimSpace(strings.Split(email, "@")[0])
	if local == "" {
		return "Salary", "Vault"
	}

	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})

	if len(parts) == 0 {
		return local, "Vault"
	}

	if len(parts) == 1 {
		return strings.Title(parts[0]), "Vault"
	}

	return strings.Title(parts[0]), strings.Title(parts[1])
}

func (s *WalletService) canAcceptDeposit(txRepo *WalletRepository, userID string, amountKobo int64) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	repo := s.repo
	if txRepo != nil {
		repo = txRepo
	}

	wallet, err := repo.GetWalletByOwnerID(userID)
	if err != nil {
		return err
	}

	currentVaultBalance := wallet.AvailableBalance + wallet.LockedBalance
	return s.subRepo.CanAddMonthlyVolume(userUUID, currentVaultBalance, amountKobo)
}

func depositLimitResponse(err error) *response.APIResponse {
	if errors.Is(err, subscription.ErrSubscriptionRequired) {
		return &response.APIResponse{
			Success: false,
			Message: "Active subscription required",
			Error:   err,
		}
	}

	if errors.Is(err, subscription.ErrPlanLimitReached) {
		return &response.APIResponse{
			Success: false,
			Message: "Maximum vault capacity reached",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: false,
		Message: "Could not validate deposit limit",
		Error:   err,
	}
}

func depositCreditErrorResponse(message string, err error) *response.APIResponse {
	if errors.Is(err, subscription.ErrSubscriptionRequired) || errors.Is(err, subscription.ErrPlanLimitReached) {
		return depositLimitResponse(err)
	}

	return &response.APIResponse{
		Success: false,
		Message: message,
		Error:   err,
	}
}

func (s *WalletService) CreditWallet(reference string, webhookAmount int64) *response.APIResponse {
	if webhookAmount <= 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid amount passed",
			Error:   "amount_must_be_greater_than_zero",
		}
	}

	var walletdeposit *WalletDeposit
	err := s.repo.ExecuteInTransaction(func(txRepo *WalletRepository) error {
		var txErr error
		walletdeposit, txErr = s.CreditWalletTx(txRepo, reference, webhookAmount)
		return txErr
	})
	if errors.Is(err, ErrDepositAmountMismatch) {
		expected := int64(0)
		if walletdeposit != nil {
			expected = walletdeposit.Amount
		}
		log.Printf("[DEPOSIT][AMOUNT_MISMATCH] reference=%s expected=%d received=%d", reference, expected, webhookAmount)
		_, _ = s.repo.UpdateDepositStatus(reference, "amount_mismatch_flagged")
		return &response.APIResponse{Success: false, Message: "Transaction amount mismatch detected", Error: "amount_mismatch_error"}
	}
	if err != nil {
		return depositCreditErrorResponse("Failed to securely execute wallet ledger update", err)
	}

	updatedDeposit, err := s.repo.GetDepositByReference(reference)
	if err == nil {
		walletdeposit = updatedDeposit
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet credited successfully",
		Data:    walletdeposit,
	}
}

// CreditWalletTx applies a verified card deposit inside the caller's
// transaction, allowing the webhook inbox row to commit atomically with it.
func (s *WalletService) CreditWalletTx(txRepo *WalletRepository, reference string, webhookAmount int64) (*WalletDeposit, error) {
	if webhookAmount <= 0 {
		return nil, ErrInvalidWalletAmount
	}
	deposit, err := txRepo.GetDepositByReferenceForUpdate(reference)
	if err != nil {
		return nil, err
	}
	if deposit.Amount != webhookAmount {
		return deposit, ErrDepositAmountMismatch
	}
	creditAmount := deposit.WalletCreditAmount
	if creditAmount <= 0 {
		creditAmount = webhookAmount
	}
	if deposit.Status == "success" {
		return deposit, nil
	}
	wallet, err := txRepo.GetWalletByOwnerIDForUpdate(deposit.UserID)
	if err != nil {
		return deposit, err
	}
	if err := s.canAcceptDeposit(txRepo, deposit.UserID, creditAmount); err != nil {
		return deposit, err
	}
	balanceBefore := wallet.LedgerBalance
	wallet.AvailableBalance += creditAmount
	wallet.LedgerBalance += creditAmount
	if _, err := txRepo.Update(wallet); err != nil {
		return deposit, err
	}
	if _, err := txRepo.UpdateDepositStatus(reference, "success"); err != nil {
		return deposit, err
	}
	ledger := &WalletLedger{
		ID: security.GenateUniqueID(), Amount: creditAmount, UserID: deposit.UserID,
		WalletID: deposit.WalletID, Currency: deposit.Currency, Type: WalletLedgerTypeCredit,
		BalanceBefore: balanceBefore, BalanceAfter: wallet.LedgerBalance,
		Reference: reference, CreatedAt: time.Now(), Source: "Paystack",
		SourceID: deposit.ID, Description: "Deposit by card",
	}
	if _, err := txRepo.LogWalletLedger(ledger); err != nil {
		return deposit, err
	}
	deposit.Status = "success"
	return deposit, nil
}

func (s *WalletService) CreditWalletWithTx(tx *gorm.DB, reference string, amount int64) (*WalletDeposit, error) {
	return s.CreditWalletTx(NewWalletRepository(tx), reference, amount)
}

func (s *WalletService) CreditDedicatedAccountTransfer(reference string, webhookAmount int64, accountNumber string) *response.APIResponse {
	if webhookAmount <= 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid amount passed",
			Error:   "amount_must_be_greater_than_zero",
		}
	}

	existingDeposit, err := s.repo.GetDepositByReference(reference)
	if err == nil && existingDeposit != nil {
		if existingDeposit.Status == "success" {
			return &response.APIResponse{
				Success: true,
				Message: "Deposit has already been processed",
				Data:    existingDeposit,
			}
		}

		return s.CreditWallet(reference, webhookAmount)
	}

	wallet, err := s.repo.GetWalletByDedicatedAccountNumber(accountNumber)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Dedicated account wallet not found",
			Error:   err.Error(),
		}
	}
	netCredit, providerFee, pricingVersion, err := s.pricing.DedicatedAccountNet(webhookAmount)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not calculate deposit fee", Error: err.Error()}
	}

	now := time.Now()
	deposit := &WalletDeposit{
		ID:          security.GenateUniqueID(),
		UserID:      wallet.UserID,
		WalletID:    wallet.ID,
		Reference:   reference,
		Amount:      webhookAmount,
		Currency:    wallet.Currency,
		Status:      "success",
		Provider:    "paystack_dedicated_account",
		GrossAmount: webhookAmount, WalletCreditAmount: netCredit, ProviderFee: providerFee, PricingVersion: pricingVersion,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = s.repo.ExecuteInTransaction(func(txRepo *WalletRepository) error {
		if err := s.canAcceptDeposit(txRepo, wallet.UserID, netCredit); err != nil {
			return err
		}

		if _, err := txRepo.CreateDeposit(deposit); err != nil {
			return err
		}

		wallet, err := txRepo.GetWalletByDedicatedAccountNumberForUpdate(accountNumber)
		if err != nil {
			return err
		}

		balanceBefore := wallet.LedgerBalance
		wallet.AvailableBalance += netCredit
		wallet.LedgerBalance += netCredit

		if _, err := txRepo.Update(wallet); err != nil {
			return err
		}

		ledger := &WalletLedger{
			ID:            security.GenateUniqueID(),
			Amount:        netCredit,
			UserID:        wallet.UserID,
			WalletID:      wallet.ID,
			Currency:      wallet.Currency,
			Type:          "credit",
			BalanceBefore: balanceBefore,
			BalanceAfter:  wallet.LedgerBalance,
			Reference:     reference,
			CreatedAt:     now,
			Source:        "Paystack Dedicated Account",
			SourceID:      deposit.ID,
			Description:   "Deposit by bank transfer",
		}

		_, err = txRepo.LogWalletLedger(ledger)
		return err
	})
	if err != nil {
		return depositCreditErrorResponse("Failed to credit dedicated account transfer", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Dedicated account transfer credited successfully",
		Data:    deposit,
	}
}

// CreditDedicatedAccountTransferWithTx makes creation of the provider deposit,
// wallet credit, ledger entry, and webhook completion one transaction.
func (s *WalletService) CreditDedicatedAccountTransferWithTx(
	tx *gorm.DB,
	reference string,
	webhookAmount int64,
	accountNumber string,
) (*WalletDeposit, error) {
	if webhookAmount <= 0 {
		return nil, ErrInvalidWalletAmount
	}
	txRepo := NewWalletRepository(tx)
	existing, err := txRepo.GetDepositByReferenceForUpdate(reference)
	if err == nil {
		if existing.Amount != webhookAmount {
			return existing, ErrDepositAmountMismatch
		}
		if existing.Status == "success" {
			return existing, nil
		}
		return s.CreditWalletTx(txRepo, reference, webhookAmount)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	wallet, err := txRepo.GetWalletByDedicatedAccountNumberForUpdate(accountNumber)
	if err != nil {
		return nil, err
	}
	netCredit, providerFee, pricingVersion, err := s.pricing.DedicatedAccountNet(webhookAmount)
	if err != nil {
		return nil, err
	}
	// Recheck after acquiring the wallet lock; another delivery may have
	// created this reference while this transaction waited.
	existing, err = txRepo.GetDepositByReferenceForUpdate(reference)
	if err == nil {
		if existing.Amount != webhookAmount {
			return existing, ErrDepositAmountMismatch
		}
		if existing.Status == "success" {
			return existing, nil
		}
		return s.CreditWalletTx(txRepo, reference, webhookAmount)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := s.canAcceptDeposit(txRepo, wallet.UserID, netCredit); err != nil {
		return nil, err
	}
	now := time.Now()
	deposit := &WalletDeposit{
		ID: security.GenateUniqueID(), UserID: wallet.UserID, WalletID: wallet.ID,
		Reference: reference, Amount: webhookAmount, Currency: wallet.Currency,
		Status: "success", Provider: "paystack_dedicated_account",
		GrossAmount: webhookAmount, WalletCreditAmount: netCredit, ProviderFee: providerFee, PricingVersion: pricingVersion,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := txRepo.CreateDeposit(deposit); err != nil {
		return nil, err
	}
	balanceBefore := wallet.LedgerBalance
	wallet.AvailableBalance += netCredit
	wallet.LedgerBalance += netCredit
	if _, err := txRepo.Update(wallet); err != nil {
		return nil, err
	}
	ledger := &WalletLedger{
		ID: security.GenateUniqueID(), Amount: netCredit, UserID: wallet.UserID,
		WalletID: wallet.ID, Currency: wallet.Currency, Type: WalletLedgerTypeCredit,
		BalanceBefore: balanceBefore, BalanceAfter: wallet.LedgerBalance,
		Reference: reference, CreatedAt: now, Source: "Paystack Dedicated Account",
		SourceID: deposit.ID, Description: "Deposit by bank transfer",
	}
	if _, err := txRepo.LogWalletLedger(ledger); err != nil {
		return nil, err
	}
	return deposit, nil
}

func (s *WalletService) GetAllDeposits(userId string) *response.APIResponse {
	walletdeposit, err := s.repo.GetWalletDepositsByUserID(userId)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to securely execute wallet ledger update",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet credited successfully",
		Data:    walletdeposit,
	}
}

func (s *WalletService) LogWalletLedger(req WalletLedger) *response.APIResponse {

	ledger, err := s.repo.LogWalletLedger(&req)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to log wallet ledger",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet ledger logged successfully",
		Data:    ledger,
	}
}
func (s *WalletService) GetMyWalletLedger(userId string) *response.APIResponse {

	ledger, err := s.repo.GetWalletLedgerByUserId(userId)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to get wallet ledger",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet ledger retrieved successfully",
		Data:    ledger,
	}
}
func (s *WalletService) GetDepositByReference(userID, reference string) *response.APIResponse {
	deposit, err := s.repo.GetDepositByReference(reference)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to get deposit",
			Error:   err.Error(),
		}
	}
	if deposit.UserID != userID {
		return &response.APIResponse{Success: false, Message: "Deposit not found", Error: "deposit_not_found"}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Deposit retrieved successfully",
		Data:    deposit,
	}
}

func (s *WalletService) DismissDeposit(userID string, reference string) *response.APIResponse {
	deposit, err := s.repo.GetDepositByReference(reference)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to get deposit",
			Error:   err.Error(),
		}
	}

	if deposit.UserID != userID {
		return &response.APIResponse{
			Success: false,
			Message: "Deposit not found",
			Error:   "deposit_not_found",
		}
	}

	if deposit.Status == "success" {
		return &response.APIResponse{
			Success: true,
			Message: "Deposit already completed",
			Data:    deposit,
		}
	}

	deposit, err = s.repo.UpdateDepositStatus(reference, "dismissed")
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not dismiss deposit",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Deposit dismissed",
		Data:    deposit,
	}
}

// ExpireStaleDeposit marks an abandoned pending deposit as expired so the
// reconciliation worker stops polling the provider for it. Called when a
// checkout link has outlived its TTL (24h) without the user paying.
func (s *WalletService) ExpireStaleDeposit(reference string) error {
	_, err := s.repo.UpdateDepositStatus(reference, "expired")
	return err
}

func (s *WalletService) VerifyDeposit(ctx context.Context, userID, reference string) *response.APIResponse {
	deposit, err := s.repo.GetDepositByReference(reference)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to get deposit",
			Error:   err.Error(),
		}
	}
	if deposit.UserID != userID {
		return &response.APIResponse{Success: false, Message: "Deposit not found", Error: "deposit_not_found"}
	}

	if deposit.Status == "success" {
		return &response.APIResponse{
			Success: true,
			Message: "Deposit already verified",
			Data: DepositVerificationResponse{
				Status:    "success",
				Reference: reference,
				Deposit:   deposit,
			},
		}
	}

	// Bug fix: the user cancelled (dismissed) this deposit or its checkout
	// link expired. Do not hit Paystack for it anymore - report it as not
	// verified so the client stops polling. Webhooks may still credit the
	// wallet if the payment actually went through.
	if deposit.Status == "dismissed" || deposit.Status == "expired" {
		return &response.APIResponse{
			Success: false,
			Message: "Deposit not verified",
			Error:   "deposit_not_verified",
		}
	}

	// Hit Paystack
	verificationResult, err := s.payment.VerifyDeposit(ctx, reference)

	// 2. Funnel execution through the idempotency machine
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Failed to process deposit request",
			Error:   err.Error(),
		}
	}

	transactionStatus := strings.ToLower(verificationResult.Data.Status)
	if !verificationResult.Status || transactionStatus != "success" {
		return &response.APIResponse{
			Success: false,
			Message: "Deposit not verified",
			Error:   "deposit_not_verified",
		}
	}

	amount := verificationResult.Data.Amount
	if amount <= 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid verified deposit amount",
			Error:   "invalid_verified_amount",
		}
	}

	creditResult := s.CreditWallet(reference, amount)
	if creditResult.Error != nil {
		if creditResult.Error == "deposit_already_processed" {
			deposit, err := s.repo.GetDepositByReference(reference)
			if err == nil {
				return &response.APIResponse{
					Success: true,
					Message: "Deposit already verified",
					Data: DepositVerificationResponse{
						Status:    "success",
						Reference: reference,
						Deposit:   deposit,
					},
				}
			}
		}

		return creditResult
	}

	verifiedDeposit, _ := s.repo.GetDepositByReference(reference)

	return &response.APIResponse{
		Success: true,
		Message: "Deposit verified successfully",
		Data: DepositVerificationResponse{
			Status:    "success",
			Reference: reference,
			Deposit:   verifiedDeposit,
		},
	}

}

func paystackPayloadData(payload interface{}) map[string]interface{} {
	root := mapFromInterface(payload)
	if root == nil {
		return nil
	}

	if data := mapFromInterface(root["data"]); data != nil {
		return data
	}

	return root
}

func mapFromInterface(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	default:
		return nil
	}
}

func stringFromMap(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}

	switch value := data[key].(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return ""
	}
}

func int64FromMap(data map[string]interface{}, key string) (int64, bool) {
	if data == nil {
		return 0, false
	}

	switch value := data[key].(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	case float64:
		return int64(value), true
	case float32:
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
