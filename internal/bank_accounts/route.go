package bankaccounts

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterRoutes(v1 fiber.Router, h *Handler) {
	group := v1.Group("/banking", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	group.Get("/banks", h.ListBanks)
	group.Post("/resolve-account", h.ResolveAccount)

	group.Get("/accounts", h.ListOwnAccounts)
	group.Post("/accounts", h.CreateOwnAccount)
	group.Patch("/accounts/:id/default", h.SetDefaultOwnAccount)
	group.Delete("/accounts/:id", h.DeleteOwnAccount)

	group.Get("/beneficiaries", h.ListBeneficiaries)
	group.Post("/beneficiaries", h.CreateBeneficiary)
	group.Delete("/beneficiaries/:id", h.DeleteBeneficiary)
}
