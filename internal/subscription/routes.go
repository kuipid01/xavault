package subscription
import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterSubscriptionRoutes(v1 fiber.Router, h *SubscriptionHandler) {
	envelopes := v1.Group("/plans/:planId/envelopes", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	envelopes.Post("/", h.CreateSubscription)
	envelopes.Get("/", h.GetActiveSubscription)
}
