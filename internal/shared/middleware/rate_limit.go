package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type RateLimitPolicy struct {
	Name   string
	Limit  int
	Window time.Duration
}

type rateBucket struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

var authRateBuckets sync.Map

// SensitiveRateLimit combines account, IP, and device identifiers. Only a hash
// is retained in memory so email addresses and device identifiers are not
// exposed through diagnostics.
func SensitiveRateLimit(policy RateLimitPolicy) fiber.Handler {
	if policy.Limit <= 0 {
		policy.Limit = 5
	}
	if policy.Window <= 0 {
		policy.Window = 15 * time.Minute
	}
	return func(c *fiber.Ctx) error {
		key := rateLimitKey(c, policy.Name)
		value, _ := authRateBuckets.LoadOrStore(key, &rateBucket{resetAt: time.Now().Add(policy.Window)})
		bucket := value.(*rateBucket)
		bucket.mu.Lock()
		now := time.Now()
		if !now.Before(bucket.resetAt) {
			bucket.count = 0
			bucket.resetAt = now.Add(policy.Window)
		}
		bucket.count++
		count := bucket.count
		resetAt := bucket.resetAt
		bucket.mu.Unlock()

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.Limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, policy.Limit-count)))
		if count > policy.Limit {
			retryAfter := int(time.Until(resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":   "rate_limit_exceeded",
				"message": "Too many attempts. Please wait before trying again.",
			})
		}
		return c.Next()
	}
}

func rateLimitKey(c *fiber.Ctx, policy string) string {
	account := ""
	var body map[string]interface{}
	if len(c.Body()) > 0 && json.Unmarshal(c.Body(), &body) == nil {
		for _, field := range []string{"email", "user_id", "device_id"} {
			if value, ok := body[field].(string); ok && strings.TrimSpace(value) != "" {
				account += ":" + strings.ToLower(strings.TrimSpace(value))
			}
		}
	}
	if userID, ok := c.Locals("user_id").(string); ok {
		account += ":" + userID
	}
	raw := policy + "|" + c.IP() + "|" + c.Get("X-Device-ID") + "|" + account
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
