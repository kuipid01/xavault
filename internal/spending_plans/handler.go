package spendingplans

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/subscription"
)

type PlanHandler struct {
	planService *PlanService
}

func NewPlanHandler(ps *PlanService) *PlanHandler {
	return &PlanHandler{planService: ps}
}

func (h *PlanHandler) CreateDraftPlan(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	var req CreatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.planService.CreateDraftPlan(userID, req)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) GetMyPlans(c *fiber.Ctx) error {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(&response.APIResponse{
			Success: false,
			Message: "Unauthorized",
			Error:   "unauthorized",
		})
	}

	query := ListPlansQuery{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 20),
	}

	result := h.planService.GetMyPlans(userID, query)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) GetPlanByID(c *fiber.Ctx) error {
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

	result := h.planService.GetPlanByID(userID, planID)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) UpdateDraftPlan(c *fiber.Ctx) error {
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

	var req UpdatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.planService.UpdateDraftPlan(userID, planID, req)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) PreviewPlan(c *fiber.Ctx) error {
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

	result := h.planService.PreviewPlan(userID, planID)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) ConfirmPlan(c *fiber.Ctx) error {
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

	var req ConfirmPlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.planService.ConfirmPlan(userID, planID, req)

	return sendPlanResponse(c, result)
}

func (h *PlanHandler) CancelPlan(c *fiber.Ctx) error {
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

	var req CancelPlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(&response.APIResponse{
			Success: false,
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
	}

	result := h.planService.CancelPlan(userID, planID, req)

	return sendPlanResponse(c, result)
}

func getUserIDFromContext(c *fiber.Ctx) (string, bool) {
	userID, ok := c.Locals("user_id").(string)

	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}

func sendPlanResponse(c *fiber.Ctx, result *response.APIResponse) error {
	if result == nil {
		return c.Status(http.StatusInternalServerError).JSON(&response.APIResponse{
			Success: false,
			Message: "Unexpected server error",
			Error:   "nil_response",
		})
	}

	statusCode := http.StatusOK

	if !result.Success {
		statusCode = getPlanErrorStatusCode(result.Error)
	}

	return c.Status(statusCode).JSON(result)
}

func getPlanErrorStatusCode(err interface{}) int {
	switch e := err.(type) {
	case string:
		switch e {
		case "unauthorized":
			return http.StatusUnauthorized
		case "plan_id_required", "invalid_parameters", "invalid_payload":
			return http.StatusBadRequest
		case "plan_not_found", "user_not_found":
			return http.StatusNotFound
		case "plan_not_in_draft_status", "unauthorized_access":
			return http.StatusForbidden
		default:
			return http.StatusBadRequest
		}

	case error:
		if errors.Is(e, ErrPlanNotFound) {
			return http.StatusNotFound
		}

		if errors.Is(e, ErrPlanNotDraft) || errors.Is(e, ErrPlanAlreadyFinal) {
			return http.StatusForbidden
		}

		if errors.Is(e, subscription.ErrSubscriptionRequired) || errors.Is(e, subscription.ErrPlanLimitReached) {
			return http.StatusForbidden
		}

		return http.StatusBadRequest

	case map[string]string:
		return http.StatusBadRequest

	default:
		return http.StatusBadRequest
	}
}
