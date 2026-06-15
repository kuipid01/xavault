package auth

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterAuthRoutes(v1 fiber.Router, h *AuthHandler) {
	auth := v1.Group("/auth")

	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/logout", h.Logout)

	auth.Post("/validate-mail", h.ValidateMail)

	auth.Post("/forget-password", h.ForgetPassword)
	auth.Post("/reset-password", h.ResetPassword)
	auth.Get("/me", h.Me)
	auth.Post("/refresh-token", h.RefreshToken)
	auth.Post("/resend-verification-mail", h.ResendVerificationMail)
	auth.Post("/change-password", h.ChangePassword)
}
