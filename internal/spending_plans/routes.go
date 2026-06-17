package spendingplans

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterPlanRoutes(v1 fiber.Router, h *PlanHandler) {
	plans := v1.Group("/plans", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	plans.Post("/", h.CreateDraftPlan)
	plans.Get("/", h.GetMyPlans)
	plans.Get("/:planId", h.GetPlanByID)
	plans.Patch("/:planId", h.UpdateDraftPlan)
	plans.Post("/:planId/preview", h.PreviewPlan)
	plans.Post("/:planId/confirm", h.ConfirmPlan)
	plans.Post("/:planId/cancel", h.CancelPlan)
}
