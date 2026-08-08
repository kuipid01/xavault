# Database migrations

Goose SQL migrations in `internal/database/migrations` are the production
source of truth. GORM maps Go values to rows; it does not own production schema
changes.

## Normal deployment

1. Back up the database and verify the backup checksum.
2. Deploy the new migration-capable artifact without starting API replicas.
3. Run `make migrate-status` and review every pending file.
4. Run `make migrate-up` once.
5. Run `make migrate-status` again and verify the schema.
6. Start or roll out API replicas.

Goose uses a PostgreSQL session advisory lock, so two migration jobs cannot
apply migrations concurrently. API startup only verifies the schema; it does
not change production tables.

## Creating migrations

Use a new, increasing six-digit version. Never edit a migration that has been
applied to a shared environment. Prefer SQL for schema changes and a Goose Go
migration only when a data transformation genuinely needs application logic.

Every SQL file starts with `-- +goose Up`. Add `-- +goose Down` only when the
reverse operation is demonstrably safe and does not discard production data.

## Rollback and forward fixes

Production defaults to forward fixes: correct a problem with a new migration.
Dropping a column or table can destroy financial history and is not an ordinary
rollback. `cmd/migrate down` is therefore disabled unless
`ALLOW_MIGRATION_DOWN=true` is set deliberately after review.

If a migration fails, Goose leaves its transaction unapplied. Fix the SQL in a
new/unshared migration or restore the pre-deployment backup. Never manually
change `goose_db_version` without an incident record and database-owner review.

## Local development

The default is `AUTO_MIGRATE=false`, matching production. A developer may set
`AUTO_MIGRATE=true` only for a disposable local database. Production config
validation rejects that setting.

## Tests

`make migrate-test` uses `XAVAULT_MIGRATION_TEST_DSN`. CI starts an empty
PostgreSQL 16 service, applies every embedded migration twice, and verifies the
required schema before running the full Go test suite.
