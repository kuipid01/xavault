run:
	go run ./cmd/api

dev:
	air

test:
	go test ./...

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	ALLOW_MIGRATION_DOWN=true go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

migrate-test:
	go test ./internal/database -run "TestVersionedMigrationsBuildFreshDatabase|TestGooseAdoptsLegacyAutoMigrateDatabase" -v
.PHONY: financial-test financial-test-race

financial-test:
	go test -p 1 -tags=integration ./internal/alerts ./internal/reconciliation ./internal/wallets ./internal/withdrawals ./internal/webhooks ./internal/auth -v

financial-test-race:
	go test -race -p 1 -tags=integration ./internal/alerts ./internal/reconciliation ./internal/wallets ./internal/withdrawals ./internal/webhooks ./internal/auth
