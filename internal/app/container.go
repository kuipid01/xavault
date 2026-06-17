package app

import (
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

type Container struct {
	Auth             *auth.Module
	Wallets          *wallets.WalletModule
	WebhookHND       *webhooks.WebhookHandler
	PlansHND         *spendingplans.PlanHandler
	EnvelopHND       *envelope.EnvelopeHandler
	WithdrawalModule *withdrawals.WithdrawalModule
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {
	idempotencyRepo := client.NewIdempotentRepository(db)
	idempotencyManager := client.NewIdempotentRequest(idempotencyRepo, true)

	walletModule := wallets.NewWalletModule(db, idempotencyManager)

	planRepo := spendingplans.NewPlanRepository(db)
	envelopeRepo := envelope.NewEnvelopeRepo(db)

	planService := spendingplans.NewPlanService(
		planRepo,
		walletModule.Repo,
		envelopeRepo,
	)

	planHandler := spendingplans.NewPlanHandler(planService)

	envelopeService := envelope.NewEnvelopeService(
		envelopeRepo,
		planRepo,
	)

	envelopeHandler := envelope.NewEnvelopeHandler(envelopeService)

	withdrawaModule := withdrawals.NewWithdrawalModule(db, idempotencyManager, planRepo, envelopeRepo, walletModule.Repo)
	mailer := mail.NewBrevoMailer()

	return &Container{
		Auth:             auth.NewModule(db, walletModule.Svc, mailer),
		Wallets:          walletModule,
		WebhookHND:       webhooks.NewWebhookHandler(walletModule.Svc),
		PlansHND:         planHandler,
		EnvelopHND:       envelopeHandler,
		WithdrawalModule: withdrawaModule,
	}
}
