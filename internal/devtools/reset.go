package devtools

import (
	"context"
	"fmt"

	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/seed"
	"gorm.io/gorm"
)

// ResetDatabase drops the public schema (all data), re-runs migrations to
// rebuild every table, and reseeds the baseline data (subscription plans).
// It backs both the QA "clear database" endpoint and the cmd/cleardb CLI.
func ResetDatabase(ctx context.Context, db *gorm.DB) error {
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error; err != nil {
		return fmt.Errorf("drop and recreate schema: %w", err)
	}

	if err := database.RunVersionedMigrations(ctx, db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	seed.Init(db)

	return nil
}
