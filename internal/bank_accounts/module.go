package bankaccounts

import (
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/shared/client"
	"gorm.io/gorm"
)

type Module struct {
	Repo *Repository
	Svc  *Service
	Hnd  *Handler
}

func NewModule(db *gorm.DB, reusableClient *client.ReusableClient) *Module {
	repo := NewRepository(db)
	paystack := paymentproviders.NewPaystackProvider(reusableClient)
	svc := NewService(repo, paystack)
	hnd := NewHandler(svc)

	return &Module{Repo: repo, Svc: svc, Hnd: hnd}
}
