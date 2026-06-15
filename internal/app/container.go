package app

import (
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type Container struct {
	Auth    *auth.Module
	Wallets *wallets.WalletModule
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {

	return &Container{
		Auth:    auth.NewModule(db),
		Wallets: wallets.NewWalletModule(db),
	}

}
