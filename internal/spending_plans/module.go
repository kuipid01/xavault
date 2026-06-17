package spendingplans

import (
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type SpendingPlanModule struct {
	Handler    *PlanHandler
	service    *PlanService
	repo       *PlanRepository
	walletRepo *wallets.WalletRepository
}

func NewSpendingPlanModule(database *gorm.DB, walletRepo *wallets.WalletRepository) *SpendingPlanModule {
	repo := NewPlanRepository(database)
	service := NewPlanService(repo, walletRepo)
	handler := NewPlanHandler(service)
	return &SpendingPlanModule{repo: repo, service: service, Handler: handler}
}
