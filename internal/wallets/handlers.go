package wallets

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type WalletHandler struct {
	svc *WalletService
}

func NewWalletHandler(svc *WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

func (h *WalletHandler) GetMyWallet(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.svc.GetMyWallet(userID)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
func (h *WalletHandler) InitiateDeposit(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	email := c.Locals("email").(string)
	var req InitiateDepositRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}

	// 1. Get the request context from Fiber and pass it into the service
	ctx := c.UserContext()
	result := h.svc.InitiateDeposit(ctx, req.IdempotencyKey, userID, email, req) // Added ctx here

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
