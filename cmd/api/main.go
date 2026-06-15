package main

import (
	"fmt"

	"github.com/gofiber/swagger"
	"github.com/kuipid01/salary-vault/internal/app"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
)

func main() {
	cfg := config.Load()

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

	//run migations
	database.RunMigrations(db)

	newContainer := app.NewContainer(db, cfg)
	fiberApp := app.New(cfg, newContainer)

	fmt.Println("Server is running on", addr)
	fiberApp.Get("/swagger/*", swagger.HandlerDefault)

	if err := fiberApp.Listen(addr); err != nil {
		fmt.Println(err)
	}
}
