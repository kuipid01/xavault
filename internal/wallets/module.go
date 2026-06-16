package wallets

import (
	"time"

	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	client "github.com/kuipid01/salary-vault/internal/shared/client"
	idempotenceClient "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"gorm.io/gorm"
)

type WalletModule struct {
	Repo *WalletRepository
	Svc  *WalletService
	Hnd  *WalletHandler
}

func NewWalletModule(db *gorm.DB, idempotencyManager *idempotenceClient.IdempotentRequest) *WalletModule {
	repo := NewWalletRepository(db)
	reusableClient := client.NewReusableClient(time.Second * 15)
	paystack := paymentproviders.NewPaystackProvider(reusableClient)

	svc := NewWalletService(repo, paystack, idempotencyManager)
	hnd := NewWalletHandler(svc)
	return &WalletModule{Repo: repo, Svc: svc, Hnd: hnd}
}
