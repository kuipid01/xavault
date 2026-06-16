package wallets

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterWalletRoutes(v1 fiber.Router, h *WalletHandler) {
	wallet := v1.Group("/wallet", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	wallet.Get("/", h.GetMyWallet)
	wallet.Post("/deposit/initiate", h.InitiateDeposit)
}
