package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/alerts"
)

type RateLimitPolicy struct {
	Name   string
	Limit  int
	Window time.Duration
}

var limiterConfiguration struct {
	sync.RWMutex
	store  RateLimitStore
	alerts *alerts.Service
}

func ConfigureSharedRateLimiter(store RateLimitStore, publisher *alerts.Service) {
	limiterConfiguration.Lock()
	defer limiterConfiguration.Unlock()
	limiterConfiguration.store = store
	limiterConfiguration.alerts = publisher
}

func SensitiveRateLimit(policy RateLimitPolicy) fiber.Handler {
	if policy.Limit <= 0 {
		policy.Limit = 5
	}
	if policy.Window <= 0 {
		policy.Window = 15 * time.Minute
	}
	return func(c *fiber.Ctx) error {
		limiterConfiguration.RLock()
		store, publisher := limiterConfiguration.store, limiterConfiguration.alerts
		limiterConfiguration.RUnlock()
		if store == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate_limiter_unavailable", "message": "Authentication protection is temporarily unavailable."})
		}
		remaining := policy.Limit
		var blockedUntil time.Time
		for _, key := range rateLimitKeys(c, policy.Name) {
			result, err := store.Increment(context.Background(), key, policy.Name, policy.Window)
			if err != nil {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate_limiter_unavailable", "message": "Authentication protection is temporarily unavailable."})
			}
			remaining = min(remaining, max(0, policy.Limit-result.Count))
			if result.Count > policy.Limit && result.ResetAt.After(blockedUntil) {
				blockedUntil = result.ResetAt
			}
		}
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.Limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !blockedUntil.IsZero() {
			retryAfter := int(time.Until(blockedUntil).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			if publisher != nil {
				_ = publisher.Publish(context.Background(), alerts.Input{Severity: alerts.SeverityWarning, EventType: "authentication_rate_limit_exceeded", Title: "Repeated authentication attempts blocked", Message: "The shared authentication limiter blocked repeated attempts", Fingerprint: "auth-rate-limit:" + policy.Name + ":" + rateLimitKeys(c, policy.Name)[0], Metadata: map[string]interface{}{"policy": policy.Name, "retry_after_seconds": retryAfter}})
			}
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate_limit_exceeded", "message": "Too many attempts. Please wait before trying again."})
		}
		return c.Next()
	}
}

func rateLimitKeys(c *fiber.Ctx, policy string) []string {
	identifiers := []string{"ip:" + c.IP()}
	var body map[string]interface{}
	if len(c.Body()) > 0 {
		_ = json.Unmarshal(c.Body(), &body)
	}
	for _, field := range []string{"email", "user_id"} {
		if value, ok := body[field].(string); ok && strings.TrimSpace(value) != "" {
			identifiers = append(identifiers, "account:"+strings.ToLower(strings.TrimSpace(value)))
		}
	}
	if userID, ok := c.Locals("user_id").(string); ok && strings.TrimSpace(userID) != "" {
		identifiers = append(identifiers, "account:"+strings.ToLower(strings.TrimSpace(userID)))
	}
	if deviceID := strings.TrimSpace(c.Get("X-Device-ID")); deviceID != "" {
		identifiers = append(identifiers, "device:"+deviceID)
	} else if value, ok := body["device_id"].(string); ok && strings.TrimSpace(value) != "" {
		identifiers = append(identifiers, "device:"+strings.TrimSpace(value))
	}
	keys := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		sum := sha256.Sum256([]byte(policy + "|" + identifier))
		keys = append(keys, hex.EncodeToString(sum[:]))
	}
	return keys
}
