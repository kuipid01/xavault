package database

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestVersionedMigrationsBuildFreshDatabase(t *testing.T) {
	dsn := os.Getenv("XAVAULT_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("XAVAULT_MIGRATION_TEST_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	resetMigrationTestSchema(t, db)
	if err := RunVersionedMigrations(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	// Running the complete migration set twice must be harmless.
	if err := RunVersionedMigrations(t.Context(), db); err != nil {
		t.Fatalf("migrations are not idempotent: %v", err)
	}
	if err := VerifyMigrationState(t.Context(), db); err != nil {
		t.Fatalf("fully migrated database was rejected: %v", err)
	}

	for _, table := range []string{
		"users", "temporary_users", "biometric_devices", "biometric_challenges",
		"security_questions", "wallets", "wallet_deposits", "wallet_ledgers",
		"spending_plans", "envelopes", "bank_accounts", "withdrawals",
		"subscription_plans", "user_subscriptions", "monthly_usages",
		"idempotent_keys", "entries", "paystack_events",
		"reconciliation_controls", "reconciliation_findings",
		"reconciliation_operator_actions", "goose_db_version",
		"operational_alerts", "operational_alert_deliveries",
		"authentication_rate_limits",
	} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("required table %q was not created", table)
		}
	}
	verifyDetailedFreshSchema(t, db)

	var applied int64
	if err := db.Table("goose_db_version").Where("is_applied = ?", true).Count(&applied).Error; err != nil {
		t.Fatal(err)
	}
	if applied != 13 {
		// Goose records its initial version zero in addition to our twelve files.
		t.Fatalf("got %d applied migration records, want 13", applied)
	}
}

func verifyDetailedFreshSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for table, columns := range map[string][]string{
		"users":           {"id", "email", "password", "withdrawal_pin_hash"},
		"wallets":         {"available_balance", "locked_balance", "ledger_balance"},
		"wallet_deposits": {"reference", "amount", "status"},
		"withdrawals":     {"reference", "provider_status", "idempotency_key"},
		"paystack_events": {"provider_event_id", "lease_expires_at", "lease_token"},
	} {
		for _, column := range columns {
			if !db.Migrator().HasColumn(table, column) {
				t.Errorf("required column %s.%s was not created", table, column)
			}
		}
	}
	for table, indexes := range map[string][]string{
		"users":           {"idx_users_email", "idx_users_nickname"},
		"wallet_deposits": {"idx_wallet_deposits_reference"},
		"wallet_ledgers":  {"idx_wallet_ledgers_reference"},
		"withdrawals":     {"idx_withdrawals_reference", "idx_withdrawals_status"},
		"paystack_events": {"paystack_events_provider_event_id_key", "idx_paystack_events_status"},
	} {
		for _, index := range indexes {
			if !db.Migrator().HasIndex(table, index) {
				t.Errorf("required index %s on %s was not created", index, table)
			}
		}
	}
	var foreignKeys, checks int64
	if err := db.Raw("SELECT count(*) FROM pg_constraint WHERE contype = 'f'").Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT count(*) FROM pg_constraint WHERE contype = 'c'").Scan(&checks).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys < 15 {
		t.Errorf("got %d foreign keys, want at least 15", foreignKeys)
	}
	if checks < 7 {
		t.Errorf("got %d check constraints, want at least 7", checks)
	}
}

func TestGooseAdoptsLegacyAutoMigrateDatabase(t *testing.T) {
	dsn := os.Getenv("XAVAULT_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("XAVAULT_MIGRATION_TEST_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	resetMigrationTestSchema(t, db)
	if err := autoMigrateDevelopmentSchema(db); err != nil {
		t.Fatalf("create legacy AutoMigrate schema: %v", err)
	}
	if err := RunVersionedMigrations(t.Context(), db); err != nil {
		t.Fatalf("adopt legacy schema with Goose: %v", err)
	}
	if err := VerifySchema(db); err != nil {
		t.Fatal(err)
	}
}

func resetMigrationTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	var name string
	if err := db.Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(name, "migration_test") && !strings.Contains(name, "goose_fresh") {
		t.Fatalf("refusing to reset non-test database %q", name)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(fmt.Errorf("reset test schema: %w", err))
	}
}
