package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var requiredTables = []string{
	"users", "temporary_users", "biometric_devices", "biometric_challenges",
	"security_questions", "wallets", "wallet_deposits", "wallet_ledgers",
	"spending_plans", "envelopes", "bank_accounts", "withdrawals",
	"subscription_plans", "user_subscriptions", "monthly_usages",
	"idempotent_keys", "entries", "paystack_events", "reconciliation_controls",
	"reconciliation_findings", "reconciliation_operator_actions",
	"operational_alerts", "operational_alert_deliveries",
	"authentication_rate_limits", "provider_fee_rules", "fee_quotes",
}

func NewMigrationProvider(db *gorm.DB) (*goose.Provider, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get SQL database: %w", err)
	}
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 60))
	if err != nil {
		return nil, fmt.Errorf("configure migration lock: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		migrations,
		goose.WithSessionLocker(locker),
		goose.WithTableName("goose_db_version"),
	)
	if err != nil {
		return nil, fmt.Errorf("configure Goose: %w", err)
	}
	return provider, nil
}

func RunVersionedMigrations(ctx context.Context, db *gorm.DB) error {
	provider, err := NewMigrationProvider(db)
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply Goose migrations: %w", err)
	}
	return VerifySchema(db)
}

func VerifySchema(db *gorm.DB) error {
	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("required table %s is missing; run the migration command", table)
		}
	}
	return nil
}

func VerifyMigrationState(ctx context.Context, db *gorm.DB) error {
	if err := VerifySchema(db); err != nil {
		return err
	}
	provider, err := NewMigrationProvider(db)
	if err != nil {
		return err
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations: %w", err)
	}
	if pending {
		return fmt.Errorf("database has pending migrations; run cmd/migrate up before starting the API")
	}
	return nil
}
