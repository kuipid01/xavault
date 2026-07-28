package wallets

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/subscription"
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

func (h *WalletHandler) EnsureDedicatedAccount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	email := c.Locals("email").(string)

	result := h.svc.EnsureDedicatedAccount(c.UserContext(), userID, email)

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
		return response.Error(c, walletErrorStatus(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *WalletHandler) GetAllDeposits(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.svc.GetAllDeposits(userID)

	if result.Error != nil {
		return response.Error(c, walletErrorStatus(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func walletErrorStatus(err interface{}) int {
	if e, ok := err.(error); ok {
		if errors.Is(e, subscription.ErrSubscriptionRequired) || errors.Is(e, subscription.ErrPlanLimitReached) {
			return fiber.StatusForbidden
		}
	}

	return fiber.StatusBadRequest
}

func (h *WalletHandler) GetMyWalletLedger(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.svc.GetMyWalletLedger(userID)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
func (h *WalletHandler) VerifyDeposit(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reference := c.Params("reference")
	if reference == "" {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", "")
	}
	ctx := c.UserContext()
	result := h.svc.VerifyDeposit(ctx, userID, reference)

	if result.Error != nil {
		return response.Error(c, walletErrorStatus(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
func (h *WalletHandler) GetDepositByReference(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reference := c.Params("reference")
	if reference == "" {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", "")
	}

	result := h.svc.GetDepositByReference(userID, reference)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *WalletHandler) DismissDeposit(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reference := c.Params("reference")
	if reference == "" {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", "")
	}

	result := h.svc.DismissDeposit(userID, reference)

	if result.Error != nil {
		return response.Error(c, walletErrorStatus(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
