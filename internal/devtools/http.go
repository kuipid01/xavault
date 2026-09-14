package devtools

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"gorm.io/gorm"
)

// RegisterRoutes mounts QA/dev-only endpoints. They are only registered when
// the API is not running as production, so a production deploy never exposes
// a database-wipe route.
func RegisterRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	if cfg == nil || strings.EqualFold(cfg.AppEnv, "production") {
		return
	}

	v1.Post("/dev/reset", func(c *fiber.Ctx) error {
		if err := ResetDatabase(c.UserContext(), db); err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "database_reset_failed", err.Error())
		}
		return response.Success(c, fiber.StatusOK, "database cleared and reseeded", nil)
	})
}
