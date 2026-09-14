package subscription

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// UserReader is a minimal interface to fetch a user's email.
// Defined here to avoid an import cycle (users → subscription → users).
type UserReader interface {
	FindById(id string) (UserInfo, error)
}

// UserInfo contains the fields the subscription handler needs from a user.
type UserInfo struct {
	Email string
}

type SubscriptionHandler struct {
	service    *SubscriptionService
	userReader UserReader
}

func NewSubscriptionHandler(
	service *SubscriptionService,
	userReader UserReader,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		service:    service,
		userReader: userReader,
	}
}

// CreateSubscription godoc
// POST /api/v1/subscriptions
// Creates a pending subscription and returns a Paystack payment URL.
func (h *SubscriptionHandler) CreateSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var req CreateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	// Fetch user email for Paystack
	userInfo, err := h.userReader.FindById(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "User not found"})
	}

	result, err := h.service.CreateSubscription(c.Context(), userID, userInfo.Email, &req)
	if err != nil {
		if errors.Is(err, ErrPlanNotUpgrade) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":   "plan_not_upgrade",
				"message": "You can only upgrade to a higher subscription plan.",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	fmt.Println("RESPONSE FROM CALL", result)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Subscription payment initiated. Complete payment to activate.",
		"data":    result,
	})
}

// DismissSubscriptionPayment godoc
// POST /api/v1/subscriptions/payment/:reference/dismiss
// Cancels an abandoned subscription checkout the user dismissed, so the
// client stops checking and the pending row is cleaned up.
func (h *SubscriptionHandler) DismissSubscriptionPayment(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	reference := strings.TrimSpace(c.Params("reference"))
	if reference == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Reference is required"})
	}

	result, err := h.service.DismissSubscriptionPayment(userID, reference)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription payment dismissed",
		"data":    result,
	})
}

// GetActiveSubscription godoc
// GET /api/v1/subscriptions/active
func (h *SubscriptionHandler) GetActiveSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	result, err := h.service.GetActiveSubscription(userID)
	fmt.Printf("HEY RESULT %v", result)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No active subscription found"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": result})
}

func (h *SubscriptionHandler) GetPlans(c *fiber.Ctx) error {
	result, err := h.service.GetPlans()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": result,
	})
}
