package withdrawals

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func NewWithdrawalRoute(v1 fiber.Router, handler *WithdrawalHandler) {
	withdrawals := v1.Group("/withdrawals", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))
	withdrawals.Post("/:planId/preview", handler.PreviewWithdrawal)
	withdrawals.Post("/:planId/request", handler.RequestWithdrawal)
	withdrawals.Get("/history", handler.GetUserWithdrawals)
	withdrawals.Get("/:planId/withdrawals", handler.GetPlanWithdrawals)
	withdrawals.Get("/:withdrawalId", handler.GetWithdrawalByID)
}
