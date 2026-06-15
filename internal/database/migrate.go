package database

import (
	"log"

	"github.com/kuipid01/salary-vault/internal/users"
	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) {
	// ClearDatabase(db)
	err := db.AutoMigrate(
		&users.User{},
	)

	if err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	log.Println("database migrations completed")
}

func ClearDatabase(db *gorm.DB) {
	err := db.Migrator().DropTable(
		&users.User{},
	)

	if err != nil {
		log.Fatalf("failed to clear database: %v", err)
	}

	log.Println("database cleared successfully")
}
