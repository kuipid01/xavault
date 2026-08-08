package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/swagger"
	"github.com/kuipid01/salary-vault/internal/app"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/seed"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Println("configuration validation failed:", err)
		os.Exit(1)
	}

	addr := ":" + cfg.AppPort
	if cfg.AppPort == "" {
		addr = ":8080"
	}
	//iniotialize db

	db, err := database.DB(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Database is connected")

	if err := database.PrepareSchema(context.Background(), db, cfg); err != nil {
		fmt.Println("database schema is not ready:", err)
		os.Exit(1)
	}

	newContainer := app.NewContainer(db, cfg)
	fiberApp := app.New(cfg, newContainer)
	workerCtx, stopWorkers := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopWorkers()
	newContainer.WebhookHND.StartRecoveryWorker(workerCtx, time.Minute)
	newContainer.ReconciliationSVC.StartWorker(workerCtx, 5*time.Minute)
	newContainer.AlertSVC.StartWorker(workerCtx, 30*time.Second)

	fmt.Println("Server is running on", addr)
	fiberApp.Get("/swagger/*", swagger.HandlerDefault)
	seed.Init(db)
	if err := fiberApp.Listen(addr); err != nil {
		fmt.Println(err)
	}
}
