package withdrawals

import "github.com/gofiber/fiber/v2"

func NewWithdrawalRoute(v1 fiber.Router, handler *WithdrawalHandler) {
	withdrawals := v1.Group("/withdrawals")
	withdrawals.Post("/:planId/preview", handler.PreviewWithdrawal)
	withdrawals.Post("/:planId/request", handler.RequestWithdrawal)
	withdrawals.Get("/:planId/withdrawals", handler.GetPlanWithdrawals)
	withdrawals.Get("/:withdrawalId", handler.GetWithdrawalByID)
}
