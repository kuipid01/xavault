package spendingplans

import (
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type SpendingPlanModule struct {
	Handler        *PlanHandler
	service        *PlanService
	Repo           *PlanRepository
	walletRepo     *wallets.WalletRepository
	envelopeReader EnvelopeAllocationReader
}

func NewSpendingPlanModule(
	database *gorm.DB,
	walletRepo *wallets.WalletRepository,
	envelopeReader EnvelopeAllocationReader,
	subRepo *subscription.SubscriptionRepository,
) *SpendingPlanModule {
	repo := NewPlanRepository(database)
	service := NewPlanService(repo, walletRepo, envelopeReader, subRepo)
	handler := NewPlanHandler(service)
	return &SpendingPlanModule{Repo: repo, service: service, Handler: handler}
}
