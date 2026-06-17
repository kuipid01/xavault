package spendingplans

import (
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

func NewSpendingPlanModule(database *gorm.DB, walletRepo *wallets.WalletRepository, envelopeReader EnvelopeAllocationReader) *SpendingPlanModule {
	repo := NewPlanRepository(database)
	service := NewPlanService(repo, walletRepo, envelopeReader)
	handler := NewPlanHandler(service)
	return &SpendingPlanModule{Repo: repo, service: service, Handler: handler}
}
