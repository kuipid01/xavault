package subscription

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type SubscriptionHandler struct {
	service         *SubscriptionService
}

func NewSubscriptionHandler(
	service *SubscriptionService,
) *SubscriptionHandler {

	return &SubscriptionHandler{
		service: service,
	}
}

func (h *SubscriptionHandler) CreateSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	// 1. Check if user already has an active subscription
	activeSub, _ := h.service.GetActiveSubscription(userID)
	if activeSub != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "You already have an active subscription"})
	}

	var req CreateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	// In a real flow, this would call the payment provider to initialize a transaction
	// or return data to the frontend to initialize the transaction.
	// We'll leave it to just validate and return the plan for now, since frontend handles Paystack pop-up.
	result, err := h.service.CreateSubscription(userID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": result})
}

func (h *SubscriptionHandler) GetActiveSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	result, err := h.service.GetActiveSubscription(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": result})
}