package middleware

import (
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

		if accessToken != "" {
			tokenString = strings.TrimSpace(accessToken)
		} else {
			if authHeader == "" {
				return response.Error(c, fiber.StatusUnauthorized, "Authorization header is required", "missing_authorization_header")
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				return response.Error(c, fiber.StatusUnauthorized, "Invalid authorization format", "invalid_authorization_format")
			}

			tokenString = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}

		if tokenString == "" {
			return response.Error(c, fiber.StatusUnauthorized, "Token is required", "missing_token")
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}

			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
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

		email, _ := claims["email"].(string)

		c.Locals("user_id", userID)
		c.Locals("email", email)

		return c.Next()
	}
}
