package envelope

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterEnvelopeRoutes(v1 fiber.Router, h *EnvelopeHandler) {
	envelopes := v1.Group("/plans/:planId/envelopes", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	envelopes.Post("/", h.CreateEnvelope)
	envelopes.Get("/", h.GetPlanEnvelopes)
	envelopes.Get("/:envelopeId", h.GetEnvelopeByID)
	envelopes.Patch("/:envelopeId", h.UpdateEnvelope)
	envelopes.Delete("/:envelopeId", h.DeleteEnvelope)
}
