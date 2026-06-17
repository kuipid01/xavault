package envelope

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type EnvelopeHandler struct {
	service *EnvelopeService
}

func NewEnvelopeHandler(s *EnvelopeService) *EnvelopeHandler {
	return &EnvelopeHandler{service: s}
}

func (h *EnvelopeHandler) CreateEnvelope(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	planID := c.Params("planId")
	if planID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		})
	}

	var req CreateEnvelopeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.service.CreateEnvelope(userID, planID, req)

	return sendEnvelopeResponse(c, result)
}

func (h *EnvelopeHandler) GetPlanEnvelopes(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	planID := c.Params("planId")
	if planID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		})
	}

	result := h.service.GetPlanEnvelopes(userID, planID)

	return sendEnvelopeResponse(c, result)
}

func (h *EnvelopeHandler) GetEnvelopeByID(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	planID := c.Params("planId")
	if planID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		})
	}

	envelopeID := c.Params("envelopeId")
	if envelopeID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		})
	}

	result := h.service.GetEnvelopeByID(userID, planID, envelopeID)

	return sendEnvelopeResponse(c, result)
}

func (h *EnvelopeHandler) UpdateEnvelope(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	planID := c.Params("planId")
	if planID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		})
	}

	envelopeID := c.Params("envelopeId")
	if envelopeID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		})
	}

	var req UpdateEnvelopeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.service.UpdateEnvelope(userID, planID, envelopeID, req)

	return sendEnvelopeResponse(c, result)
}

func (h *EnvelopeHandler) DeleteEnvelope(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	planID := c.Params("planId")
	if planID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		})
	}

	envelopeID := c.Params("envelopeId")
	if envelopeID == "" {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		})
	}

	result := h.service.DeleteEnvelope(userID, planID, envelopeID)

	return sendEnvelopeResponse(c, result)
}

func getUserIDFromContext(c *fiber.Ctx) (string, bool) {
	userID, ok := c.Locals("user_id").(string)

	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}

func sendEnvelopeResponse(c *fiber.Ctx, result *response.APIResponse) error {
	if result == nil {
		return c.Status(http.StatusInternalServerError).JSON(&response.APIResponse{
			Success: false,
			Message: "Unexpected server error",
			Error:   "nil_response",
		})
	}

	statusCode := http.StatusOK

	if !result.Success {
		statusCode = getEnvelopeErrorStatusCode(result.Error)
	}

	return c.Status(statusCode).JSON(result)
}

func getEnvelopeErrorStatusCode(err interface{}) int {
	switch e := err.(type) {
	case string:
		switch e {
		case "unauthorized":
			return http.StatusUnauthorized

		case "plan_id_required",
			"envelope_id_required",
			"invalid_parameters",
			"invalid_payload":
			return http.StatusBadRequest

		case "plan_not_found",
			"envelope_not_found":
			return http.StatusNotFound

		case "plan_not_in_draft_status",
			"unauthorized_access",
			"envelope_not_editable":
			return http.StatusForbidden

		default:
			return http.StatusBadRequest
		}

	case error:
		if errors.Is(e, ErrEnvelopeNotFound) {
			return http.StatusNotFound
		}

		if errors.Is(e, ErrPlanNotFound) {
			return http.StatusNotFound
		}

		if errors.Is(e, ErrPlanNotDraft) {
			return http.StatusForbidden
		}

		if errors.Is(e, ErrEnvelopeAmountExceeded) ||
			errors.Is(e, ErrEnvelopeInvalidDateRange) ||
			errors.Is(e, ErrEnvelopeParentNotFound) ||
			errors.Is(e, ErrEnvelopeCannotBeParentOfItself) {
			return http.StatusBadRequest
		}

		return http.StatusBadRequest

	case map[string]string:
		return http.StatusBadRequest

	default:
		return http.StatusBadRequest
	}
}
