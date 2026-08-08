package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: go run ./cmd/migrate <up|status|down>")
	}
	cfg := config.Load()
	db, err := database.DB(cfg)
	if err != nil {
		fatal("connect to database: %v", err)
	}
	provider, err := database.NewMigrationProvider(db)
	if err != nil {
		fatal("configure migrations: %v", err)
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			fatal("migration up failed: %v", err)
		}
		for _, result := range results {
			fmt.Println(result)
		}
		if err := database.VerifySchema(db); err != nil {
			fatal("schema verification failed: %v", err)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			fatal("migration status failed: %v", err)
		}
		for _, status := range statuses {
			fmt.Printf("%06d %-8s %s\n", status.Source.Version, status.State, status.Source.Path)
		}
	case "down":
		if os.Getenv("ALLOW_MIGRATION_DOWN") != "true" {
			fatal("down migrations are disabled; set ALLOW_MIGRATION_DOWN=true after reviewing data-loss risk")
		}
		result, err := provider.Down(ctx)
		if err != nil {
			fatal("migration down failed: %v", err)
		}
		fmt.Println(result)
	default:
		fatal("unknown migration command %q", os.Args[1])
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
