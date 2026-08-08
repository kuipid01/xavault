package middleware

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type memoryRateLimitStore struct {
	mu     sync.Mutex
	counts map[string]int
	reset  time.Time
}

func (s *memoryRateLimitStore) Increment(_ context.Context, key, _ string, window time.Duration) (RateLimitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts == nil || time.Now().After(s.reset) {
		s.counts = map[string]int{}
		s.reset = time.Now().Add(window)
	}
	s.counts[key]++
	return RateLimitResult{Count: s.counts[key], ResetAt: s.reset}, nil
}

func TestSensitiveRateLimitCannotBeBypassedByRotatingDeviceID(t *testing.T) {
	ConfigureSharedRateLimiter(&memoryRateLimitStore{}, nil)
	app := fiber.New()
	app.Post("/login", SensitiveRateLimit(RateLimitPolicy{Name: "test-login", Limit: 2, Window: time.Minute}), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for attempt, device := range []string{"device-a", "device-b", "device-c"} {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"user@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Device-ID", device)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		want := fiber.StatusOK
		if attempt == 2 {
			want = fiber.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d: got status %d, want %d", attempt+1, resp.StatusCode, want)
		}
	}
}
