package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

func (h *AuthHandler) EnrollBiometricDevice(c *fiber.Ctx) error {
	var req EnrollBiometricDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	result := h.authService.EnrollBiometricDevice(c.Locals("user_id").(string), req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}
	return response.Success(c, fiber.StatusCreated, result.Message, result.Data)
}

func (h *AuthHandler) ListBiometricDevices(c *fiber.Ctx) error {
	result := h.authService.ListBiometricDevices(c.Locals("user_id").(string))
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}
	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) RevokeBiometricDevice(c *fiber.Ctx) error {
	result := h.authService.RevokeBiometricDevice(c.Locals("user_id").(string), c.Params("deviceId"))
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}
	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) CreateBiometricChallenge(c *fiber.Ctx) error {
	var req BiometricChallengeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	result := h.authService.CreateBiometricChallenge(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusUnauthorized, result.Message, result.Error)
	}
	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) VerifyBiometricChallenge(c *fiber.Ctx) error {
	var req VerifyBiometricRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	result := h.authService.VerifyBiometricChallenge(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusUnauthorized, result.Message, result.Error)
	}
	data := result.Data.(map[string]interface{})
	SetAuthCookies(c, data["access_token"].(string), data["refresh_token"].(string))
	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
