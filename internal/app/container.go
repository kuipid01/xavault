package app

import (
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
	"gorm.io/gorm"
)

type Container struct {
	Auth       *auth.Module
	Wallets    *wallets.WalletModule
	WebhookHND *webhooks.WebhookHandler
	PlansHND   *spendingplans.PlanHandler
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {
	idempotencyRepo := client.NewIdempotentRepository(db)
	idempotencyManager := client.NewIdempotentRequest(idempotencyRepo, true)
	walletModule := wallets.NewWalletModule(db, idempotencyManager)
	planModule := spendingplans.NewSpendingPlanModule(db, walletModule.Repo)
	mailer := mail.NewBrevoMailer()
	return &Container{
		Auth:       auth.NewModule(db, walletModule.Svc, mailer),
		Wallets:    walletModule,
		WebhookHND: webhooks.NewWebhookHandler(walletModule.Svc),
		PlansHND:   planModule.Handler,
	}

}
