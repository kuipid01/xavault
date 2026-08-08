package database

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/kuipid01/salary-vault/internal/auth"
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/reconciliation"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/waitlist"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

func PrepareSchema(ctx context.Context, db *gorm.DB, cfg *config.Config) error {
	if cfg.AutoMigrate {
		if strings.EqualFold(cfg.AppEnv, "production") {
			return fmt.Errorf("AUTO_MIGRATE cannot be enabled in production")
		}
		if err := autoMigrateDevelopmentSchema(db); err != nil {
			return fmt.Errorf("development AutoMigrate failed: %w", err)
		}
		log.Println("development AutoMigrate completed")
		return nil
	}
	return VerifyMigrationState(ctx, db)
}

func autoMigrateDevelopmentSchema(db *gorm.DB) error {
	return db.AutoMigrate(
		&users.User{},
		&users.SecurityQuestion{},
		&auth.TemporaryUser{},
		&auth.BiometricDevice{},
		&auth.BiometricChallenge{},
		&wallets.Wallet{},
		&wallets.WalletDeposit{},
		&client.IdempotentKey{},
		&wallets.WalletLedger{},
		&spendingplans.SpendingPlan{},
		&envelope.Envelope{},
		&bankaccounts.BankAccount{},
		&withdrawals.Withdrawal{},
		&waitlist.Entry{},
		&subscription.SubscriptionPlan{},
		&subscription.UserSubscription{},
		&subscription.MonthlyUsage{},
		&webhooks.PaystackEvent{},
		&reconciliation.Control{},
		&reconciliation.Finding{},
		&reconciliation.OperatorAction{},
	)
}

func ClearDatabase(db *gorm.DB) {
	err := db.Migrator().DropTable(
		&users.User{},
		&users.SecurityQuestion{},
		&auth.TemporaryUser{},
		&auth.BiometricChallenge{},
		&auth.BiometricDevice{},
		&wallets.Wallet{},
		&wallets.WalletDeposit{},
		&client.IdempotentKey{},
		&wallets.WalletLedger{},
		&bankaccounts.BankAccount{},
	)

	if err != nil {
		log.Fatalf("failed to clear database: %v", err)
	}

	log.Println("database cleared successfully")
}
