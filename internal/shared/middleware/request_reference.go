package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

const SupportReferenceLocal = "support_reference"
const SupportReferenceHeader = "X-Support-Reference"

func RequestReference() fiber.Handler {
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		random := make([]byte, 4)
		if _, err := rand.Read(random); err != nil {
			copy(random, []byte(fmt.Sprintf("%08d", time.Now().UnixNano())))
		}
		reference := fmt.Sprintf("XAV-%s-%s", time.Now().UTC().Format("20060102"), hex.EncodeToString(random))
		c.Locals(SupportReferenceLocal, reference)
		c.Set(SupportReferenceHeader, reference)
		err := c.Next()
		level := "info"
		if err != nil || c.Response().StatusCode() >= fiber.StatusBadRequest {
			level = "error"
		}
		log.Printf(
			`{"level":%q,"event":"http_request","support_reference":%q,"status":%d,"method":%q,"path":%q,"latency_ms":%d}`,
			level, reference, c.Response().StatusCode(), c.Method(), c.Path(), time.Since(startedAt).Milliseconds(),
		)
		return err
	}
}

func GetSupportReference(c *fiber.Ctx) string {
	reference, _ := c.Locals(SupportReferenceLocal).(string)
	return reference
}
