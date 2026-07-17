package wallets

import (
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	client "github.com/kuipid01/salary-vault/internal/shared/client"
	idempotenceClient "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"gorm.io/gorm"
)

type WalletModule struct {
	Repo *WalletRepository
	Svc  *WalletService
	Hnd  *WalletHandler
}

func NewWalletModule(
	db *gorm.DB,
	idempotencyManager *idempotenceClient.IdempotentRequest,
	reusableClient *client.ReusableClient,
	subRepo *subscription.SubscriptionRepository,
) *WalletModule {
	repo := NewWalletRepository(db)

	paystack := paymentproviders.NewPaystackProvider(reusableClient)

	svc := NewWalletService(repo, paystack, idempotencyManager, subRepo)
	hnd := NewWalletHandler(svc)
	return &WalletModule{Repo: repo, Svc: svc, Hnd: hnd}
}
