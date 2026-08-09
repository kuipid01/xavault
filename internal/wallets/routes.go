package wallets

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
)

func RegisterWalletRoutes(v1 fiber.Router, h *WalletHandler) {
	wallet := v1.Group("/wallet", middleware.Protected(os.Getenv("JWT_ACCESS_SECRET")))

	wallet.Get("/", h.GetMyWallet)
	wallet.Post("/dedicated-account", h.EnsureDedicatedAccount)
	wallet.Post("/deposit/initiate", h.InitiateDeposit)
	wallet.Post("/deposit/quote", h.QuoteDeposit)
	wallet.Get("/deposits", h.GetAllDeposits)
	wallet.Get("/deposit/:reference", h.GetDepositByReference)
	wallet.Post("/deposit/:reference/dismiss", h.DismissDeposit)
	wallet.Get("/ledger", h.GetMyWalletLedger)
	wallet.Post("/deposit/:reference/verify", h.VerifyDeposit)
}
