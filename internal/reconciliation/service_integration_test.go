//go:build integration

package reconciliation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/reconciliation"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/wallets"
)

type fakeProvider struct {
	depositResponse    *paymentproviders.VerifyTransactionResponse
	depositErr         error
	depositVerifies    int
	withdrawalVerifies int
	transferInitiates  int
}

func (f *fakeProvider) InitiateDeposit(context.Context, *paymentproviders.DepositBody) (*paymentproviders.DepositResponse, error) {
	return nil, nil
}
func (f *fakeProvider) VerifyDeposit(context.Context, string) (*paymentproviders.VerifyTransactionResponse, error) {
	f.depositVerifies++
	return f.depositResponse, f.depositErr
}
func (f *fakeProvider) ListBanks(context.Context) (*paymentproviders.ListBanksResponse, error) {
	return nil, nil
}
func (f *fakeProvider) ResolveBankAccount(context.Context, string, string) (*paymentproviders.ResolveBankAccountResponse, error) {
	return nil, nil
}
func (f *fakeProvider) CreateCustomer(context.Context, *paymentproviders.CreateCustomerBody) (*paymentproviders.CreateCustomerResponse, error) {
	return nil, nil
}
func (f *fakeProvider) CreateDedicatedAccount(context.Context, *paymentproviders.CreateDedicatedAccountBody) (*paymentproviders.CreateDedicatedAccountResponse, error) {
	return nil, nil
}
func (f *fakeProvider) CreateTransferRecipient(context.Context, *paymentproviders.CreateTransferRecipientBody) (*paymentproviders.CreateTransferRecipientResponse, error) {
	return nil, nil
}
func (f *fakeProvider) InitiateTransfer(context.Context, *paymentproviders.InitiateTransferBody) (*paymentproviders.InitiateTransferResponse, error) {
	f.transferInitiates++
	return nil, nil
}
func (f *fakeProvider) VerifyWithdrawal(context.Context, string) (*paymentproviders.VerifyTransferResponse, error) {
	f.withdrawalVerifies++
	return nil, errors.New("provider timeout")
}

func TestCleanFinancialStateDoesNotHalt(t *testing.T) {
	db := testsupport.FinancialDB(t)
	_, wallet := testsupport.SeedUserWallet(t, db, 1000, 0, 1000)
	if err := db.Create(&wallets.WalletLedger{
		ID: uuid.NewString(), UserID: wallet.UserID, WalletID: wallet.ID,
		Currency: "NGN", Reference: "CLEAN-CREDIT", Type: "credit", Source: "deposit",
		SourceID: uuid.NewString(), Amount: 1000, BalanceBefore: 0, BalanceAfter: 1000, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := reconciliation.NewService(db, nil, nil, nil).RunFinancialCheck(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Findings != 0 || result.ProcessingHalted {
		t.Fatalf("clean state produced findings: %+v", result)
	}
}

func TestInvariantFailureHaltsAndResumeIsAudited(t *testing.T) {
	db := testsupport.FinancialDB(t)
	_, _ = testsupport.SeedUserWallet(t, db, 1000, 0, 900)
	service := reconciliation.NewService(db, nil, nil, nil)
	result, err := service.RunFinancialCheck(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.ProcessingHalted || result.Findings == 0 {
		t.Fatalf("expected halt, got %+v", result)
	}

	control, findings, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !control.ProcessingHalted || len(findings) == 0 {
		t.Fatal("halt/finding was not persisted")
	}
	if err := service.Resume(context.Background(), "operator-42", "verified corrective ledger entry"); err != nil {
		t.Fatal(err)
	}
	control, _, err = service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if control.ProcessingHalted {
		t.Fatal("processing remained halted after authorized resume")
	}
	var action reconciliation.OperatorAction
	if err := db.Where("operator_id = ?", "operator-42").First(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.Details != "verified corrective ledger entry" {
		t.Fatalf("unexpected audit details %q", action.Details)
	}
}

func TestSuccessfulDepositWithoutLedgerIsDetected(t *testing.T) {
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 1000, 0, 1000)
	deposit := wallets.WalletDeposit{
		ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Reference: "MISSING-LEDGER",
		Amount: 1000, Currency: "NGN", Status: "success", Provider: "paystack",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&deposit).Error; err != nil {
		t.Fatal(err)
	}
	result, err := reconciliation.NewService(db, nil, nil, nil).RunFinancialCheck(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.ProcessingHalted {
		t.Fatal("missing deposit ledger did not halt processing")
	}
	var count int64
	if err := db.Model(&reconciliation.Finding{}).Where("code = ?", "deposit_ledger_mismatch").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d deposit mismatch findings, want 1", count)
	}
}

func TestProviderTimeoutLeavesPendingDepositUntouched(t *testing.T) {
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, 0, 0)
	deposit := wallets.WalletDeposit{ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Reference: "TIMEOUT-DEPOSIT", Amount: 2500, Currency: "NGN", Status: "pending", Provider: "paystack", CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour)}
	if err := db.Create(&deposit).Error; err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{depositErr: errors.New("provider timeout")}
	walletRepo := wallets.NewWalletRepository(db)
	walletService := wallets.NewWalletService(walletRepo, provider, client.NewIdempotentRequest(client.NewIdempotentRepository(db), true), subscription.NewSubscriptionRepository(db))
	if err := reconciliation.NewService(db, provider, walletService, nil).ReconcileStuck(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, err := walletRepo.GetDepositByReference(deposit.Reference)
	if err != nil {
		t.Fatal(err)
	}
	currentWallet, err := walletRepo.GetWalletByID(wallet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if provider.depositVerifies != 1 || stored.Status != "pending" || currentWallet.LedgerBalance != 0 {
		t.Fatalf("timeout mutated money state: verifies=%d deposit=%s wallet=%+v", provider.depositVerifies, stored.Status, currentWallet)
	}
}

func TestAmbiguousDepositVerificationHaltsWithoutCrediting(t *testing.T) {
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, 0, 0)
	deposit := wallets.WalletDeposit{ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Reference: "AMBIGUOUS-DEPOSIT", Amount: 2500, Currency: "NGN", Status: "pending", Provider: "paystack", CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour)}
	if err := db.Create(&deposit).Error; err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{depositResponse: &paymentproviders.VerifyTransactionResponse{Status: true, Data: paymentproviders.VerifyTransactionData{Reference: deposit.Reference, Status: "success", Amount: 5000, Currency: "NGN"}}}
	walletRepo := wallets.NewWalletRepository(db)
	walletService := wallets.NewWalletService(walletRepo, provider, client.NewIdempotentRequest(client.NewIdempotentRepository(db), true), subscription.NewSubscriptionRepository(db))
	service := reconciliation.NewService(db, provider, walletService, nil)
	if err := service.ReconcileStuck(context.Background()); err != nil {
		t.Fatal(err)
	}
	control, _, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	currentWallet, err := walletRepo.GetWalletByID(wallet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !control.ProcessingHalted || currentWallet.LedgerBalance != 0 {
		t.Fatalf("ambiguous verification was not safely halted: control=%+v wallet=%+v", control, currentWallet)
	}
}
