package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/config"
)

func New(cfg *config.Config, container *Container) *fiber.App {

	fiberApp := fiber.New(fiber.Config{
		AppName: cfg.AppName,
	})
	RegisterRoutes(fiberApp, cfg, container)

	return fiberApp
}
