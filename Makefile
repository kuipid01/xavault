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
	migrate -path migrations -database "postgres://salary_vault:salary_vault_password@localhost:5432/salary_vault?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://salary_vault:salary_vault_password@localhost:5432/salary_vault?sslmode=disable" down