//go:build integration

package wallets_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

func walletService(t *testing.T) (*wallets.WalletService, *wallets.WalletRepository, *gorm.DB, string, string) {
	t.Helper()
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, 0, 0)
	testsupport.SeedActiveSubscription(t, db, user.ID)
	repo := wallets.NewWalletRepository(db)
	idempotency := client.NewIdempotentRequest(client.NewIdempotentRepository(db), true)
	service := wallets.NewWalletService(repo, nil, idempotency, subscription.NewSubscriptionRepository(db))
	return service, repo, db, user.ID, wallet.ID
}

func TestConcurrentDuplicateDepositCreditsExactlyOnce(t *testing.T) {
	service, repo, db, userID, walletID := walletService(t)
	reference := "CONCURRENT-DEPOSIT-1"
	if _, err := repo.CreateDeposit(&wallets.WalletDeposit{
		ID: uuid.NewString(), UserID: userID, WalletID: walletID, Reference: reference,
		Amount: 2500, Currency: "NGN", Status: "pending", Provider: "paystack",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() { defer wg.Done(); service.CreditWallet(reference, 2500) }()
	}
	wg.Wait()
	wallet, err := repo.GetWalletByID(walletID)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalance != 2500 || wallet.LedgerBalance != 2500 {
		t.Fatalf("duplicate credit changed balance more than once: %+v", wallet)
	}
	var ledgerCount int64
	if err := db.Model(&wallets.WalletLedger{}).Where("reference = ?", reference).Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 {
		t.Fatalf("got %d ledger entries, want 1", ledgerCount)
	}
}

func TestDepositAmountMismatchNeverCreditsWallet(t *testing.T) {
	service, repo, _, userID, walletID := walletService(t)
	reference := "AMOUNT-MISMATCH-1"
	if _, err := repo.CreateDeposit(&wallets.WalletDeposit{
		ID: uuid.NewString(), UserID: userID, WalletID: walletID, Reference: reference,
		Amount: 2500, Currency: "NGN", Status: "pending", Provider: "paystack",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	result := service.CreditWallet(reference, 5000)
	if result.Success {
		t.Fatal("amount mismatch was accepted")
	}
	wallet, err := repo.GetWalletByID(walletID)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalance != 0 || wallet.LedgerBalance != 0 {
		t.Fatalf("mismatch credited wallet: %+v", wallet)
	}
	deposit, err := repo.GetDepositByReference(reference)
	if err != nil {
		t.Fatal(err)
	}
	if deposit.Status != "amount_mismatch_flagged" {
		t.Fatalf("unexpected status %q", deposit.Status)
	}
}
