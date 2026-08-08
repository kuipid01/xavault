package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	TokenType string `json:"type"`
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
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   userID,
			Issuer:    t.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expireAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(t.AccessSecret)
}

func (t *TokenService) ValidateToken(tokenString string, expectedType string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}

		return t.AccessSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	if expectedType != "" && claims.TokenType != expectedType {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
