package withdrawals

import (
	bankaccounts "github.com/kuipid01/salary-vault/internal/bank_accounts"
	"github.com/kuipid01/salary-vault/internal/envelope"
	"github.com/kuipid01/salary-vault/internal/funds"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/pricing"
	"github.com/kuipid01/salary-vault/internal/shared/client"
	idempotenceClient "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type WithdrawalModule struct {
	Repo *WithdrawalRepository
	Svc  *WithdrawalService
	Hnd  *WithdrawalHandler
}

func NewWithdrawalModule(db *gorm.DB, idempotencyManager *idempotenceClient.IdempotentRequest, planRepo *spendingplans.PlanRepository, envelopeRepo *envelope.EnvelopeRepo, walletRepo *wallets.WalletRepository, bankAccountRepo *bankaccounts.Repository, userRepo *users.UserRepository, reusableClient *client.ReusableClient, fundsRepo *funds.Repository, extras ...interface{}) *WithdrawalModule {
	repo := NewWithdrawalRepository(db)
	paymentproviders := paymentproviders.NewPaystackProvider(reusableClient)
	pricingService := pricing.NewService(db)
	subRepo := subscription.NewSubscriptionRepository(db)
	for _, extra := range extras {
		switch v := extra.(type) {
		case *pricing.Service:
			pricingService = v
		case *subscription.SubscriptionRepository:
			subRepo = v
		}
	}
	svc := NewWithdrawalService(repo, planRepo, envelopeRepo, walletRepo, bankAccountRepo, userRepo, paymentproviders, idempotencyManager, fundsRepo, pricingService, subRepo)
	hnd := NewWithdrawalHandler(svc)
	return &WithdrawalModule{Repo: repo, Svc: svc, Hnd: hnd}
}
