package auth

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterAuthRoutes(v1 fiber.Router, h *AuthHandler) {
	auth := v1.Group("/auth")

	auth.Post("/register", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "register", Limit: 5, Window: time.Hour}), h.Register)
	auth.Post("/register/start", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "register_start", Limit: 5, Window: time.Hour}), h.StartRegistration)
	auth.Post("/register/verify-mail", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "register_verify", Limit: 8, Window: 15 * time.Minute}), h.VerifyRegistrationMail)
	auth.Post("/login", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "login", Limit: 8, Window: 15 * time.Minute}), h.Login)
	auth.Post("/logout", h.Logout)

	auth.Post("/validate-mail", h.ValidateMail)
	auth.Post("/resend-verification-mail", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "otp_resend", Limit: 3, Window: 30 * time.Minute}), h.ResendVerificationMail)

	auth.Post("/forget-password", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "forgot_password", Limit: 3, Window: 30 * time.Minute}), h.ForgetPassword)
	auth.Post("/reset-password", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "reset_password", Limit: 6, Window: 30 * time.Minute}), h.ResetPassword)
	auth.Post("/refresh-token", h.RefreshToken)
	auth.Post("/biometric/challenge", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "biometric_challenge", Limit: 10, Window: 15 * time.Minute}), h.CreateBiometricChallenge)
	auth.Post("/biometric/verify", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "biometric_verify", Limit: 10, Window: 15 * time.Minute}), h.VerifyBiometricChallenge)

	auth.Use(middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	auth.Get("/me", h.Me)
	auth.Post("/logout-all-devices", h.LogoutAllDevices)
	auth.Get("/profile", h.Me)
	auth.Post("/biometric/devices", h.EnrollBiometricDevice)
	auth.Get("/biometric/devices", h.ListBiometricDevices)
	auth.Delete("/biometric/devices/:deviceId", h.RevokeBiometricDevice)
	auth.Put("/profile", h.UpdateProfile)
	auth.Get("/profile/nickname/check", h.CheckNickname)
	auth.Post("/profile/image", h.UploadProfileImage)

	auth.Get("/withdrawal-pin/status", h.WithdrawalPinStatus)
	auth.Post("/withdrawal-pin/verify", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "withdrawal_pin", Limit: 5, Window: 30 * time.Minute}), h.VerifyWithdrawalPin)
	auth.Post("/withdrawal-pin", h.SetWithdrawalPin)
	auth.Post("/withdrawal-pin/forgot", h.ForgotWithdrawalPin)
	auth.Post("/withdrawal-pin/reset", h.ResetWithdrawalPin)

	auth.Get("/security-questions", h.ListSecurityQuestions)
	auth.Put("/security-questions", h.SaveSecurityQuestions)

	auth.Post("/change-password", h.ChangePassword)
}
