package database

import (
	"log"

	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
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
	)

	if err != nil {
		log.Fatalf("failed to clear database: %v", err)
	}

	log.Println("database cleared successfully")
}
