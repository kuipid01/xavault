package webhooks

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterWebhooksRoutes(v1 fiber.Router, h *WebhookHandler) {
	webhooks := v1.Group("/webhooks")

	webhooks.Post("/paystack", h.PaystackWebhook)
}
