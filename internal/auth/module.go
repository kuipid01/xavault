package auth

import (
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	"github.com/kuipid01/salary-vault/internal/users"
	"gorm.io/gorm"
)

type Module struct {
	Repo *users.UserRepository
	Svc  *AuthService
	Hnd  *AuthHandler
}

func NewModule(db *gorm.DB) *Module {
	brevoMailer := mail.NewBrevoMailer()
	repo := users.NewUserRepository(db)
	tokenService := NewTokenService()
	svc := NewAuthService(repo, brevoMailer, tokenService)

	hnd := NewAuthHandler(svc)
	return &Module{Repo: repo, Svc: svc, Hnd: hnd}
}
