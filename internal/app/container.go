package app

import (
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/config"
	"gorm.io/gorm"
)

type Container struct {
	Auth *auth.Module
}

func NewContainer(db *gorm.DB, cfg *config.Config) *Container {

	return &Container{
		Auth: auth.NewModule(db),
	}

}
