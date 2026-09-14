package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/devtools"
)

func main() {
	fmt.Println("Starting database clear and seed process...")

	// 1. Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Printf("configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Connect to the database
	db, err := database.DB(cfg)
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Database is connected.")

	// 3-5. Drop all tables, re-run migrations, and reseed baseline data
	fmt.Println("Clearing database, running migrations, and seeding...")
	if err := devtools.ResetDatabase(context.Background(), db); err != nil {
		fmt.Printf("failed to reset database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Database has been cleared and seeded successfully for audit testing.")
}
