package reconciliation

import (
	"crypto/subtle"
	"errors"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		expected := strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
		provided := strings.TrimSpace(c.Get("X-Admin-Key"))
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "admin_authorization_required"})
		}
		c.Locals("operator_id", c.Get("X-Operator-ID", "admin"))
		return c.Next()
	}
}

func RegisterRoutes(v1 fiber.Router, handler *Handler) {
	group := v1.Group("/admin/reconciliation", AdminOnly())
	group.Get("/", handler.status)
	group.Post("/run", handler.run)
	group.Post("/resume", handler.resume)
}

func (h *Handler) status(c *fiber.Ctx) error {
	control, findings, err := h.service.Status(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "reconciliation_status_failed"})
	}
	return c.JSON(fiber.Map{"control": control, "findings": findings})
}

func (h *Handler) run(c *fiber.Ctx) error {
	result, err := h.service.RunFinancialCheck(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "reconciliation_run_failed"})
	}
	return c.JSON(result)
}

func (h *Handler) resume(c *fiber.Ctx) error {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.Reason) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "reason_required"})
	}
	operatorID, _ := c.Locals("operator_id").(string)
	if err := h.service.Resume(c.UserContext(), operatorID, input.Reason); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "resume_failed"})
	}
	return c.JSON(fiber.Map{"status": "resumed"})
}

// FinancialSafety blocks state-changing money endpoints when reconciliation
// has detected a critical invariant failure.
func FinancialSafety(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		path := c.Path()
		financial := strings.Contains(path, "/wallet") ||
			strings.Contains(path, "/withdrawals") ||
			strings.Contains(path, "/plans") ||
			strings.Contains(path, "/envelopes") ||
			strings.Contains(path, "/subscriptions")
		if !financial {
			return c.Next()
		}
		var control Control
		if err := db.Select("processing_halted", "reason").First(&control, 1).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Next()
			}
			return c.Status(503).JSON(fiber.Map{"error": "financial_safety_check_unavailable"})
		}
		if control.ProcessingHalted {
			return c.Status(503).JSON(fiber.Map{
				"error":   "financial_processing_temporarily_halted",
				"message": "Financial processing is temporarily unavailable while balances are verified.",
			})
		}
		return c.Next()
	}
}
