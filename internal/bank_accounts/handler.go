package bankaccounts

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListBanks(c *fiber.Ctx) error {
	result := h.svc.ListBanks(c.UserContext())
	return writeResponse(c, result)
}

func (h *Handler) ResolveAccount(c *fiber.Ctx) error {
	var req ResolveBankAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}

	result := h.svc.ResolveAccount(c.UserContext(), req)
	return writeResponse(c, result)
}

func (h *Handler) ListOwnAccounts(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	result := h.svc.ListAccounts(userID, AccountOwnerTypeSelf)
	return writeResponse(c, result)
}

func (h *Handler) CreateOwnAccount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req CreateBankAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}

	result := h.svc.CreateAccount(c.UserContext(), userID, AccountOwnerTypeSelf, req)
	return writeResponse(c, result)
}

func (h *Handler) SetDefaultOwnAccount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	result := h.svc.SetDefault(userID, c.Params("id"), AccountOwnerTypeSelf)
	return writeResponse(c, result)
}

func (h *Handler) DeleteOwnAccount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	result := h.svc.DeleteAccount(userID, c.Params("id"), AccountOwnerTypeSelf)
	return writeResponse(c, result)
}

func (h *Handler) ListBeneficiaries(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	result := h.svc.ListAccounts(userID, AccountOwnerTypeBeneficiary)
	return writeResponse(c, result)
}

func (h *Handler) CreateBeneficiary(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req CreateBankAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", err)
	}

	result := h.svc.CreateAccount(c.UserContext(), userID, AccountOwnerTypeBeneficiary, req)
	return writeResponse(c, result)
}

func (h *Handler) DeleteBeneficiary(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	result := h.svc.DeleteAccount(userID, c.Params("id"), AccountOwnerTypeBeneficiary)
	return writeResponse(c, result)
}

func writeResponse(c *fiber.Ctx, result *response.APIResponse) error {
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
