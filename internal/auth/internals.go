package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	"golang.org/x/crypto/bcrypt"
)

func CheckPasswordHash(password string, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(password),
	)

	return err == nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func SetAuthCookies(c *fiber.Ctx, accessToken string, refreshToken string) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HTTPOnly: true,
		Secure:   security.IsProduction(),
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   15 * 60,
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   security.IsProduction(),
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
	})
}
func RemoveAuthCookies(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   security.IsProduction(),
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   security.IsProduction(),
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func GenerateAllToken(userID string, email string, tokenService *TokenService) (string, string, error) {
	accessToken, err := tokenService.GenerateAccessToken(userID, email, "access")
	if err != nil {
		return "", "", err
	}
	refreshToken, err := tokenService.GenerateAccessToken(userID, email, "refresh")
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}
