package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/auth"
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/reconciliation"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/waitlist"
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
	v1.Use(reconciliation.FinancialSafety(container.db))
	auth.RegisterAuthRoutes(v1, container.Auth.Hnd)
	wallets.RegisterWalletRoutes(v1, container.Wallets.Hnd)
	webhooks.RegisterWebhooksRoutes(v1, container.WebhookHND)
	spendingplans.RegisterPlanRoutes(v1, container.PlansHND)
	envelope.RegisterEnvelopeRoutes(v1, container.EnvelopHND)
	bankaccounts.RegisterRoutes(v1, container.BankAccounts.Hnd)
	withdrawals.NewWithdrawalRoute(v1, container.WithdrawalModule.Hnd)
	waitlist.RegisterRoutes(v1, container.Waitlist.Hnd)
	subscription.RegisterSubscriptionRoutes(v1, container.SubscriptionHND)
	reconciliation.RegisterRoutes(v1, container.ReconciliationHND)
	v1.Get("/", func(c *fiber.Ctx) error {

		return response.Success(c, fiber.StatusOK, "Salary Vault API V1", nil)
	})
}
