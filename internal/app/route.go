package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
)

type handlers struct {
	authHandler *auth.AuthHandler
}

func RegisterRoutes(app *fiber.App, cfg *config.Config, container *Container) {

	app.Get("/health", func(c *fiber.Ctx) error {

		return response.Success(c, fiber.StatusOK, "Salary Vault is healthy", nil)
	})
	api := app.Group("/api")
	v1 := api.Group("/v1")
	auth.RegisterAuthRoutes(v1, container.Auth.Hnd)
	wallets.RegisterWalletRoutes(v1, container.Wallets.Hnd)
	webhooks.RegisterWebhooksRoutes(v1, container.WebhookHND)
	spendingplans.RegisterPlanRoutes(v1, container.PlansHND)
	envelope.RegisterEnvelopeRoutes(v1, container.EnvelopHND)
	withdrawals.NewWithdrawalRoute(v1, container.WithdrawalModule.Hnd)
	v1.Get("/", func(c *fiber.Ctx) error {

		return response.Success(c, fiber.StatusOK, "Salary Vault API V1", nil)
	})
}
