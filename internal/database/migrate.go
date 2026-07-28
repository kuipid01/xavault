package database

import (
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

func RunMigrations(db *gorm.DB, cfg *config.Config) {
	if strings.EqualFold(cfg.AppEnv, "production") {
		if err := RunVersionedMigrations(db); err != nil {
			log.Fatalf("failed to run versioned database migrations: %v", err)
		}
		log.Println("versioned database migrations completed")
		return
	}
	// ClearDatabase(db)
	err := db.AutoMigrate(
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

	if err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	log.Println("database migrations completed")
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
