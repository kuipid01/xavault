package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type AuthHandler struct {
	authService *AuthService
}

func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Register(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Login(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	if result.Data != nil {
		dataMap := result.Data.(map[string]interface{})
		accessToken := dataMap["access_token"].(string)
		refreshToken := dataMap["refresh_token"].(string)
		SetAuthCookies(c, accessToken, refreshToken)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ValidateMail(c *fiber.Ctx) error {
	var req ValidateMailRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ValidateMail(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ForgetPassword(c *fiber.Ctx) error {
	var req ForgetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ForgetPassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ResetPassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Logout(req)
	if result.Data != nil {

		RemoveAuthCookies(c)
	}
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {

	userID := c.Locals("user_id").(string)

	result := h.authService.Me(MeRequest{
		UserID: userID,
	})

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	if req.RefreshToken == "" {
		req.RefreshToken = c.Cookies("refresh_token")
	}

	result := h.authService.RefreshToken(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	if result.Data != nil {
		dataMap := result.Data.(fiber.Map)
		accessToken := dataMap["access_token"].(string)
		refreshToken := dataMap["refresh_token"].(string)
		SetAuthCookies(c, accessToken, refreshToken)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResendVerificationMail(c *fiber.Ctx) error {
	var req ResendVerificationMailRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ResendVerificationMail(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ChangePassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) WithdrawalPinStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.authService.WithdrawalPinStatus(WithdrawalPinStatusRequest{
		UserID: userID,
	})

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) SetWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req SetWithdrawalPinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.SetWithdrawalPin(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ForgotWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.authService.ForgotWithdrawalPin(ForgotWithdrawalPinRequest{
		UserID: userID,
	})
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResetWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req ResetWithdrawalPinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.ResetWithdrawalPin(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
