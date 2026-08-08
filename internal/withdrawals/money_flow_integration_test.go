//go:build integration

package withdrawals_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

func withdrawalFixture(t *testing.T) (*gorm.DB, *withdrawals.WithdrawalService, *wallets.WalletRepository, *withdrawals.Withdrawal) {
	t.Helper()
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, 5000, 5000)
	now := time.Now()
	plan := spendingplans.SpendingPlan{
		ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Name: "Withdrawal test plan",
		Amount: 5000, Currency: "NGN", Status: spendingplans.PlanStatusActive,
		FundingStatus: spendingplans.PlanFundingStatusLocked, StartDate: now.Add(-time.Hour), EndDate: now.Add(time.Hour),
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	w := &withdrawals.Withdrawal{
		ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, PlanID: plan.ID,
		SourceType: withdrawals.WithdrawalSourcePlanUnboxed, Status: withdrawals.WithdrawalStatusProcessing,
		Amount: 5000, Currency: "NGN", Reference: "WITHDRAWAL-FLOW-1", Provider: "paystack",
	}
	if err := db.Create(w).Error; err != nil {
		t.Fatal(err)
	}
	walletRepo := wallets.NewWalletRepository(db)
	service := withdrawals.NewWithdrawalService(withdrawals.NewWithdrawalRepository(db), nil, nil, walletRepo, nil, nil, nil, nil, nil)
	return db, service, walletRepo, w
}

func TestWithdrawalCompletionIsExactlyOnce(t *testing.T) {
	db, service, walletRepo, withdrawal := withdrawalFixture(t)
	if result := service.CompleteProviderWithdrawal(withdrawal.Reference, "success"); !result.Success {
		t.Fatalf("first completion failed: %+v", result)
	}
	if result := service.CompleteProviderWithdrawal(withdrawal.Reference, "success"); !result.Success {
		t.Fatalf("duplicate completion was not idempotent: %+v", result)
	}
	wallet, err := walletRepo.GetWalletByID(withdrawal.WalletID)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.LockedBalance != 0 || wallet.LedgerBalance != 0 {
		t.Fatalf("completion debited more than once: %+v", wallet)
	}
	var count int64
	if err := db.Model(&wallets.WalletLedger{}).Where("source_id = ? AND type = ?", withdrawal.ID, wallets.WalletLedgerTypeWithdrawalDebit).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d withdrawal debit ledgers, want 1", count)
	}
}

func TestPaidWithdrawalReversalRefundsExactlyOnce(t *testing.T) {
	db, service, walletRepo, withdrawal := withdrawalFixture(t)
	if result := service.CompleteProviderWithdrawal(withdrawal.Reference, "success"); !result.Success {
		t.Fatalf("completion failed: %+v", result)
	}
	if result := service.FailProviderWithdrawal(withdrawal.Reference, "reversed", "provider reversed transfer"); !result.Success {
		t.Fatalf("reversal failed: %+v", result)
	}
	if result := service.FailProviderWithdrawal(withdrawal.Reference, "reversed", "duplicate webhook"); !result.Success {
		t.Fatalf("duplicate reversal was not idempotent: %+v", result)
	}
	wallet, err := walletRepo.GetWalletByID(withdrawal.WalletID)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalance != 0 || wallet.LockedBalance != 5000 || wallet.LedgerBalance != 5000 {
		t.Fatalf("reversal did not restore the original locked funds exactly once: %+v", wallet)
	}
	var count int64
	if err := db.Model(&wallets.WalletLedger{}).Where("source_id = ? AND type = ?", withdrawal.ID, wallets.WalletLedgerTypeRefund).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d refund ledgers, want 1", count)
	}
}
