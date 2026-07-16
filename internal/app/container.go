package app

import (
	"fmt"
	"time"

	"github.com/kuipid01/salary-vault/internal/auth"
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	clientNetwork "github.com/kuipid01/salary-vault/internal/shared/client"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/waitlist"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

// userReaderAdapter adapts *users.UserRepository to the subscription.UserReader interface.
// This avoids an import cycle (subscription → users → subscription).
type userReaderAdapter struct {
	repo *users.UserRepository
}

func (a *userReaderAdapter) FindById(id string) (subscription.UserInfo, error) {
	u, err := a.repo.FindById(id)
	if err != nil {
		return subscription.UserInfo{}, err
	}
	if u == nil {
		return subscription.UserInfo{}, fmt.Errorf("user %s not found", id)
	}
	return subscription.UserInfo{Email: u.Email}, nil
}

type Container struct {
	Auth             *auth.Module
	Wallets          *wallets.WalletModule
	WebhookHND       *webhooks.WebhookHandler
	PlansHND         *spendingplans.PlanHandler
	EnvelopHND       *envelope.EnvelopeHandler
	WithdrawalModule *withdrawals.WithdrawalModule
	BankAccounts     *bankaccounts.Module
	Waitlist         *waitlist.Module
	SubscriptionHND  *subscription.SubscriptionHandler
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {
	reusableClient := clientNetwork.NewReusableClient(time.Second * 15)
	idempotencyRepo := client.NewIdempotentRepository(db)
	idempotencyManager := client.NewIdempotentRequest(idempotencyRepo, true)

	walletModule := wallets.NewWalletModule(db, idempotencyManager, reusableClient)

	planRepo := spendingplans.NewPlanRepository(db)
	envelopeRepo := envelope.NewEnvelopeRepo(db)

	// --- Shared user repo (used by subscriptions, withdrawals) ---
	userRepo := users.NewUserRepository(db)

	// --- Subscription ---
	subRepo := subscription.NewSubscriptionRepository(db)
	paystackProvider := paymentproviders.NewPaystackProvider(reusableClient)
	subService := subscription.NewSubscriptionService(subRepo, paystackProvider)
	subHandler := subscription.NewSubscriptionHandler(subService, &userReaderAdapter{repo: userRepo})

	// --- Spending Plans (needs subRepo for container-limit enforcement) ---
	planService := spendingplans.NewPlanService(
		planRepo,
		walletModule.Repo,
		envelopeRepo,
		subRepo,
	)
	planHandler := spendingplans.NewPlanHandler(planService)

	envelopeService := envelope.NewEnvelopeService(envelopeRepo, planRepo, subRepo)
	envelopeHandler := envelope.NewEnvelopeHandler(envelopeService)

	bankAccountsModule := bankaccounts.NewModule(db, reusableClient)
	withdrawaModule := withdrawals.NewWithdrawalModule(
		db, idempotencyManager, planRepo, envelopeRepo,
		walletModule.Repo, bankAccountsModule.Repo, userRepo, reusableClient,
	)
	mailer := mail.NewBrevoMailer()

	return &Container{
		Auth:             auth.NewModule(db, walletModule.Svc, mailer),
		Wallets:          walletModule,
		WebhookHND:       webhooks.NewWebhookHandler(walletModule.Svc, withdrawaModule.Svc, subService),
		PlansHND:         planHandler,
		EnvelopHND:       envelopeHandler,
		WithdrawalModule: withdrawaModule,
		BankAccounts:     bankAccountsModule,
		Waitlist:         waitlist.NewModule(db),
		SubscriptionHND:  subHandler,
	}
}
