//go:build integration

package envelope_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/funds"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

// envelopeCapacityFixture builds an active plan of planAmount kobo with a
// fresh subscription and returns the services under test. All dates are
// midnight-aligned because envelope requests carry date-only strings.
func envelopeCapacityFixture(t *testing.T, planAmount int64) (*gorm.DB, *envelope.EnvelopeService) {
	t.Helper()
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, planAmount, planAmount)
	testsupport.SeedActiveSubscription(t, db, user.ID)
	today := time.Now()
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	plan := spendingplans.SpendingPlan{
		ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Name: "Capacity plan",
		Amount: planAmount, Currency: "NGN", Status: spendingplans.PlanStatusActive,
		FundingStatus: spendingplans.PlanFundingStatusLocked,
		StartDate: midnight.AddDate(0, 0, -2), EndDate: midnight.AddDate(0, 0, 3),
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	service := envelope.NewEnvelopeService(
		envelope.NewEnvelopeRepo(db),
		spendingplans.NewPlanRepository(db),
		subscription.NewSubscriptionRepository(db),
		funds.NewRepository(db),
	)
	return db, service
}

func capacityTestUserID(t *testing.T, db *gorm.DB, planID string) string {
	t.Helper()
	var userID string
	if err := db.Raw("SELECT user_id FROM spending_plans WHERE id = ?", planID).Scan(&userID).Error; err != nil || userID == "" {
		t.Fatalf("could not resolve plan owner: %v", err)
	}
	return userID
}

func seedEnvelope(t *testing.T, db *gorm.DB, planID, userID string, amount int64) envelope.Envelope {
	t.Helper()
	today := time.Now()
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	e := envelope.Envelope{
		ID: uuid.NewString(), UserID: userID, PlanID: planID, Name: "Env",
		Amount: amount, Status: envelope.EnvelopeStatusActive,
		UnboxedBalancePolicy: envelope.EnvelopeUnboxedPolicyFlexible,
		StartDate:            midnight.AddDate(0, 0, -1), EndDate: midnight.AddDate(0, 0, 2),
	}
	if err := db.Create(&e).Error; err != nil {
		t.Fatal(err)
	}
	return e
}

func seedWithdrawal(t *testing.T, db *gorm.DB, planID, userID string, envelopeID *string, sourceType string, amount int64) {
	t.Helper()
	var walletID string
	if err := db.Raw("SELECT id FROM wallets WHERE user_id = ? LIMIT 1", userID).Scan(&walletID).Error; err != nil || walletID == "" {
		t.Fatalf("could not resolve wallet for user: %v", err)
	}
	w := withdrawals.Withdrawal{
		ID: uuid.NewString(), UserID: userID, WalletID: walletID, PlanID: planID,
		EnvelopeID: envelopeID, SourceType: sourceType, Status: withdrawals.WithdrawalStatusPaid,
		Amount: amount, TotalDeduction: amount, Currency: "NGN", Reference: "WITHDRAWAL-" + uuid.NewString(),
	}
	if err := db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
}

func capacityRequest(name string, amount int64, parentID *string) envelope.CreateEnvelopeRequest {
	today := time.Now()
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return envelope.CreateEnvelopeRequest{
		Name: name, Amount: amount, ParentEnvelopeID: parentID,
		StartDate: midnight.Format("2006-01-02"),
		EndDate:   midnight.AddDate(0, 0, 1).Format("2006-01-02"),
		UnboxedBalancePolicy: "flexible",
	}
}

func rejectedForCapacity(t *testing.T, result *response.APIResponse) {
	t.Helper()
	if result.Success {
		t.Fatalf("expected rejection, got success")
	}
	err, ok := result.Error.(error)
	if !ok || !errors.Is(err, envelope.ErrEnvelopeAmountExceeded) {
		t.Fatalf("expected ErrEnvelopeAmountExceeded, got: %#v", result.Error)
	}
}

func TestExhaustedPlanRejectsNewEnvelope(t *testing.T) {
	db, service := envelopeCapacityFixture(t, 1000)
	var plan spendingplans.SpendingPlan
	if err := db.First(&plan).Error; err != nil {
		t.Fatal(err)
	}
	userID := capacityTestUserID(t, db, plan.ID)

	seedEnvelope(t, db, plan.ID, userID, 500)
	seedWithdrawal(t, db, plan.ID, userID, nil, withdrawals.WithdrawalSourcePlanUnboxed, 500)
	seedWithdrawal(t, db, plan.ID, userID, nil, withdrawals.WithdrawalSourceEnvelope, 500)

	result := service.CreateEnvelope(userID, plan.ID, capacityRequest("Ghost envelope", 500, nil))
	rejectedForCapacity(t, result)
}

func TestPlanLevelWithdrawalReducesEnvelopeCapacity(t *testing.T) {
	db, service := envelopeCapacityFixture(t, 1000)
	var plan spendingplans.SpendingPlan
	if err := db.First(&plan).Error; err != nil {
		t.Fatal(err)
	}
	userID := capacityTestUserID(t, db, plan.ID)

	seedWithdrawal(t, db, plan.ID, userID, nil, withdrawals.WithdrawalSourcePlanUnboxed, 400)

	// Only 600 of the original 1000 may still be allocated.
	if result := service.CreateEnvelope(userID, plan.ID, capacityRequest("Fits", 600, nil)); !result.Success {
		t.Fatalf("expected 600-kobo envelope to fit after a 400 withdrawal: %+v", result)
	}

	rejectedForCapacity(t, service.CreateEnvelope(userID, plan.ID, capacityRequest("Does not fit", 601, nil)))
}

func TestEnvelopeWithdrawalBlocksChildAllocation(t *testing.T) {
	db, service := envelopeCapacityFixture(t, 1000)
	var plan spendingplans.SpendingPlan
	if err := db.First(&plan).Error; err != nil {
		t.Fatal(err)
	}
	userID := capacityTestUserID(t, db, plan.ID)

	parent := seedEnvelope(t, db, plan.ID, userID, 500)
	seedWithdrawal(t, db, plan.ID, userID, &parent.ID, withdrawals.WithdrawalSourceEnvelope, 500)

	parentID := parent.ID
	result := service.CreateEnvelope(userID, plan.ID, capacityRequest("Child of drained parent", 100, &parentID))
	rejectedForCapacity(t, result)

	// Wallet sanity: no money moved in these tests.
	var w wallets.Wallet
	if err := db.First(&w).Error; err != nil {
		t.Fatal(err)
	}
	if w.LockedBalance != 1000 || w.AvailableBalance != 0 {
		t.Fatalf("test fixture unexpectedly moved wallet funds: %+v", w)
	}
}
