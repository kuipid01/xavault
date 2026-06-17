package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService struct {
	AccessSecret []byte
	Issuer       string
}

func NewTokenService() *TokenService {
	return &TokenService{
		AccessSecret: []byte(os.Getenv("JWT_ACCESS_SECRET")),
		Issuer:       os.Getenv("APP_NAME"),
	}
}

type AccessTokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (t *TokenService) GenerateAccessToken(userID string, email string, tokenType string) (string, error) {
	now := time.Now()
	var expireAt time.Time

	if tokenType == TokenTypeAccess {
		expireAt = now.Add(AuthTimeAccess * time.Minute)
	} else if tokenType == TokenTypeRefresh {
		expireAt = now.Add(AuthTimeRefresh * 24 * time.Hour)
	} else {
		expireAt = now.Add(AuthTimeAccess * time.Minute)
	}
	claims := AccessTokenClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    t.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expireAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(t.AccessSecret)
}
