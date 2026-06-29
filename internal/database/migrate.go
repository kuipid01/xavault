package database

import (
	"log"

	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/envelope"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/waitlist"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) {
	// ClearDatabase(db)
	err := db.AutoMigrate(
		&users.User{},
		&wallets.Wallet{},
		&wallets.WalletDeposit{},
		&client.IdempotentKey{},
		&wallets.WalletLedger{},
		&spendingplans.SpendingPlan{},
		&envelope.Envelope{},
		&bankaccounts.BankAccount{},
		&withdrawals.Withdrawal{},
		&waitlist.Entry{},
	)

	if err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	log.Println("database migrations completed")
}

func ClearDatabase(db *gorm.DB) {
	err := db.Migrator().DropTable(
		&users.User{},
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
