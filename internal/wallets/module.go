package wallets

import "gorm.io/gorm"

type WalletModule struct {
	Repo *WalletRepository
	Svc  *WalletService
	Hnd  *WalletHandler
}

func NewWalletModule(db *gorm.DB) *WalletModule {
	repo := NewWalletRepository(db)
	svc := NewWalletService(repo)
	hnd := NewWalletHandler(svc)
	return &WalletModule{Repo: repo, Svc: svc, Hnd: hnd}
}
