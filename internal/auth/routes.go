package auth

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterAuthRoutes(v1 fiber.Router, h *AuthHandler) {
	auth := v1.Group("/auth")

	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/logout", h.Logout)

	auth.Post("/validate-mail", h.ValidateMail)
	auth.Post("/resend-verification-mail", h.ResendVerificationMail)

	auth.Post("/forget-password", h.ForgetPassword)
	auth.Post("/reset-password", h.ResetPassword)
	auth.Post("/refresh-token", h.RefreshToken)

	auth.Use(middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	auth.Get("/me", h.Me)
	auth.Get("/withdrawal-pin/status", h.WithdrawalPinStatus)
	auth.Post("/withdrawal-pin", h.SetWithdrawalPin)
	auth.Post("/withdrawal-pin/forgot", h.ForgotWithdrawalPin)
	auth.Post("/withdrawal-pin/reset", h.ResetWithdrawalPin)

	auth.Post("/change-password", h.ChangePassword)
}
