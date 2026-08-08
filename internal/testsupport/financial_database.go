//go:build integration

package testsupport

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func FinancialDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("XAVAULT_FINANCIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("XAVAULT_FINANCIAL_TEST_DSN is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(name, "financial_test") {
		t.Fatalf("refusing to reset non-financial-test database %q", name)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.RunVersionedMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func SeedActiveSubscription(t *testing.T, db *gorm.DB, userID string) {
	t.Helper()
	uid := uuid.MustParse(userID)
	plan := subscription.SubscriptionPlan{
		BaseModel: subscription.BaseModel{ID: uuid.New()}, Code: subscription.PlanPremium,
		Name: "Financial Test", Currency: "NGN", MaxMonthlyVolumeKobo: 1_000_000_000,
		MaxLockedPlans: 100, MaxFlexibleEnvelopes: 100, MaxEmergencyVaults: 100,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sub := subscription.UserSubscription{
		BaseModel: subscription.BaseModel{ID: uuid.New()}, UserID: uid, PlanID: plan.ID,
		Status: subscription.SubActive, CurrentPeriodStart: now.Add(-time.Hour), CurrentPeriodEnd: now.Add(24 * time.Hour),
	}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
}

func SeedUserWallet(t *testing.T, db *gorm.DB, available, locked, ledger int64) (users.User, wallets.Wallet) {
	t.Helper()
	userID := uuid.NewString()
	user := users.User{
		ID: userID, Email: userID + "@example.test", Password: "hash",
		FirstName: "Financial", LastName: "Test", Country: "NG", PhoneNumber: "08000000000",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	wallet := wallets.Wallet{
		ID: uuid.NewString(), UserID: userID, AvailableBalance: available,
		LockedBalance: locked, LedgerBalance: ledger, Currency: "NGN",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	return user, wallet
}
