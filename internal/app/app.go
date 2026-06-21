package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/kuipid01/salary-vault/internal/config"
)

func New(cfg *config.Config, container *Container) *fiber.App {

	fiberApp := fiber.New(fiber.Config{
		AppName: cfg.AppName,
	})
	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))
	RegisterRoutes(fiberApp, cfg, container)

	return fiberApp
}
