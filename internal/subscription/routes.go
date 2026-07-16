package subscription

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterSubscriptionRoutes(v1 fiber.Router, h *SubscriptionHandler) {
	subs := v1.Group("/subscriptions", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))
	subs.Get("/plans", h.GetPlans)
	subs.Post("/", h.CreateSubscription)
	subs.Get("/active", h.GetActiveSubscription)
}
