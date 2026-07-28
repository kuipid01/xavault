package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

func Protected(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		accessToken := c.Cookies("access_token")

		var tokenString string

		if authHeader != "" {
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return response.Error(c, fiber.StatusUnauthorized, "Invalid authorization format", "invalid_authorization_format")
			}
			tokenString = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		} else {
			if accessToken == "" {
				return response.Error(c, fiber.StatusUnauthorized, "Authorization header is required", "missing_authorization_header")
			}
			tokenString = strings.TrimSpace(accessToken)
		}

		if tokenString == "" {
			return response.Error(c, fiber.StatusUnauthorized, "Token is required", "missing_token")
		}

		secrets := []string{jwtSecret}
		if previous := strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET_PREVIOUS")); previous != "" && previous != jwtSecret {
			secrets = append(secrets, previous)
		}
		var token *jwt.Token
		var err error
		for _, secret := range secrets {
			token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err == nil && token.Valid {
				break
			}
		}
		if err != nil || token == nil || !token.Valid {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired token", "invalid_token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid token claims", "invalid_token_claims")
		}

		userID, ok := claims["user_id"].(string)

		if !ok || userID == "" {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid user ID in token", "invalid_user_id")
		}

		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid token type", "invalid_token_type")
		}

		email, _ := claims["email"].(string)

		c.Locals("user_id", userID)
		c.Locals("email", email)

		return c.Next()
	}
}
