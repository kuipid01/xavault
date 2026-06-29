package app

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/auth"
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	clientNetwork "github.com/kuipid01/salary-vault/internal/shared/client"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/waitlist"
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
	BankAccounts     *bankaccounts.Module
	Waitlist         *waitlist.Module
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {
	reusableClient := clientNetwork.NewReusableClient(time.Second * 15)
	idempotencyRepo := client.NewIdempotentRepository(db)
	idempotencyManager := client.NewIdempotentRequest(idempotencyRepo, true)

	walletModule := wallets.NewWalletModule(db, idempotencyManager, reusableClient)

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
	bankAccountsModule := bankaccounts.NewModule(db, reusableClient)
	userRepo := users.NewUserRepository(db)

	withdrawaModule := withdrawals.NewWithdrawalModule(db, idempotencyManager, planRepo, envelopeRepo, walletModule.Repo, bankAccountsModule.Repo, userRepo, reusableClient)
	mailer := mail.NewBrevoMailer()

	return &Container{
		Auth:             auth.NewModule(db, walletModule.Svc, mailer),
		Wallets:          walletModule,
		WebhookHND:       webhooks.NewWebhookHandler(walletModule.Svc),
		PlansHND:         planHandler,
		EnvelopHND:       envelopeHandler,
		WithdrawalModule: withdrawaModule,
		BankAccounts:     bankAccountsModule,
		Waitlist:         waitlist.NewModule(db),
	}
}
