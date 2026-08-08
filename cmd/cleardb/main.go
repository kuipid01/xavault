package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/seed"
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

	// 3. Drop all tables by recreating the public schema
	// This ensures a clean slate, removing any orphaned records or constraints.
	fmt.Println("Dropping all data to ensure a clean slate...")
	err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error
	if err != nil {
		fmt.Printf("failed to drop and recreate schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Schema cleared successfully.")

	// 4. Run migrations to rebuild tables
	fmt.Println("Running migrations...")
	if err := database.RunVersionedMigrations(context.Background(), db); err != nil {
		fmt.Printf("failed to run migrations: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Migrations completed.")

	// 5. Seed predictable baseline data (Plans, etc.)
	// Note: Explicitly not creating any test users as requested.
	fmt.Println("Seeding base plans and predictable state...")
	seed.Init(db)
	fmt.Println("Seeding completed.")

	fmt.Println("\n✅ Database has been cleared and seeded successfully for audit testing.")
}
