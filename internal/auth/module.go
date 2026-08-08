package auth

import (
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"gorm.io/gorm"
)

type Module struct {
	Repo     *users.UserRepository
	AuthRepo *AuthRepository
	Svc      *AuthService
	Hnd      *AuthHandler
}

func NewModule(db *gorm.DB, walletService *wallets.WalletService, brevoMailer *mail.BrevoMailer) *Module {
	repo := users.NewUserRepository(db)
	authRepo := NewAuthRepository(db)
	tokenService := NewTokenService()
	svc := NewAuthService(repo, authRepo, brevoMailer, tokenService, walletService)
	svc.SetRegistrationDB(db)

	hnd := NewAuthHandler(svc)
	return &Module{Repo: repo, AuthRepo: authRepo, Svc: svc, Hnd: hnd}
}
