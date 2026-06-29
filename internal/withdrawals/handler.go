package withdrawals

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type WithdrawalHandler struct {
	svc *WithdrawalService
}

func NewWithdrawalHandler(svc *WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{svc: svc}
}

func (h *WithdrawalHandler) PreviewWithdrawal(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", "unauthorized")
	}

	planID := c.Params("planId")
	if planID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Plan ID is required", "plan_id_required")
	}

	var req PreviewWithdrawalRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}

	result := h.svc.PreviewWithdrawal(userID, planID, req)

	if result.Error != nil {
		return response.Error(c, getWithdrawalStatusCode(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *WithdrawalHandler) RequestWithdrawal(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", "unauthorized")
	}

	planID := c.Params("planId")
	if planID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Plan ID is required", "plan_id_required")
	}

	var req RequestWithdrawalRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}
	// 1. Get the request context from Fiber and pass it into the service
	ctx := c.UserContext()
	email := c.Locals("email").(string)
	result := h.svc.RequestWithdrawal(ctx, userID, email, planID, req, req.IdempotencyKey)

	if result.Error != nil {
		return response.Error(c, getWithdrawalStatusCode(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *WithdrawalHandler) GetPlanWithdrawals(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", "unauthorized")
	}

	planID := c.Params("planId")
	if planID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Plan ID is required", "plan_id_required")
	}

	result := h.svc.GetPlanWithdrawals(userID, planID)

	if result.Error != nil {
		return response.Error(c, getWithdrawalStatusCode(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *WithdrawalHandler) GetWithdrawalByID(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", "unauthorized")
	}

	withdrawalID := c.Params("withdrawalId")
	if withdrawalID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Withdrawal ID is required", "withdrawal_id_required")
	}

	result := h.svc.GetWithdrawalByID(userID, withdrawalID)

	if result.Error != nil {
		return response.Error(c, getWithdrawalStatusCode(result.Error), result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func getWithdrawalStatusCode(err interface{}) int {
	switch err {
	case ErrWithdrawalNotFound:
		return fiber.StatusNotFound

	case ErrPlanNotFound:
		return fiber.StatusNotFound

	case ErrEnvelopeNotFound:
		return fiber.StatusNotFound

	case ErrWalletNotFound:
		return fiber.StatusNotFound

	case ErrPlanNotActive:
		return fiber.StatusForbidden

	case ErrInsufficientWithdrawable:
		return fiber.StatusBadRequest

	case ErrInvalidWithdrawalSource:
		return fiber.StatusBadRequest

	case ErrInvalidWithdrawalAmount:
		return fiber.StatusBadRequest

	case ErrEnvelopeRequired:
		return fiber.StatusBadRequest

	case ErrEnvelopeNotAllowed:
		return fiber.StatusBadRequest

	case ErrInvalidWithdrawalDestination:
		return fiber.StatusBadRequest

	case ErrWithdrawalRequestInProgress:
		return fiber.StatusConflict

	case ErrWithdrawalAlreadyProcessing:
		return fiber.StatusConflict

	case ErrWithdrawalPinNotSet:
		return fiber.StatusForbidden

	case ErrInvalidWithdrawalPin:
		return fiber.StatusUnauthorized

	default:
		return fiber.StatusBadRequest
	}
}
