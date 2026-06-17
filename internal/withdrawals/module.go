package withdrawals

import (
	"github.com/kuipid01/salary-vault/internal/envelope"
	idempotenceClient "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type WithdrawalModule struct {
	Repo *WithdrawalRepository
	Svc  *WithdrawalService
	Hnd  *WithdrawalHandler
}

func NewWithdrawalModule(db *gorm.DB, idempotencyManager *idempotenceClient.IdempotentRequest, planRepo *spendingplans.PlanRepository, envelopeRepo *envelope.EnvelopeRepo, walletRepo *wallets.WalletRepository) *WithdrawalModule {
	repo := NewWithdrawalRepository(db)
	svc := NewWithdrawalService(repo, planRepo, envelopeRepo, walletRepo)
	hnd := NewWithdrawalHandler(svc)
	return &WithdrawalModule{Repo: repo, Svc: svc, Hnd: hnd}
}
