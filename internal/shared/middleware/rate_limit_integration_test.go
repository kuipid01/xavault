//go:build integration

package middleware_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/middleware"
	"github.com/kuipid01/salary-vault/internal/testsupport"
)

type failingStore struct{}

func (failingStore) Increment(context.Context, string, string, time.Duration) (middleware.RateLimitResult, error) {
	return middleware.RateLimitResult{}, errors.New("database unavailable")
}

func TestPostgresLimiterIsSharedAcrossApplicationInstances(t *testing.T) {
	db := testsupport.FinancialDB(t)
	storeA := middleware.NewPostgresRateLimitStore(db)
	storeB := middleware.NewPostgresRateLimitStore(db)
	newApp := func(store middleware.RateLimitStore) *fiber.App {
		middleware.ConfigureSharedRateLimiter(store, nil)
		app := fiber.New()
		app.Post("/login", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "shared-login", Limit: 2, Window: time.Minute}), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		return app
	}
	appA, appB := newApp(storeA), newApp(storeB)
	for attempt, app := range []*fiber.App{appA, appB, appA} {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"same@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		want := fiber.StatusOK
		if attempt == 2 {
			want = fiber.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d returned %d, want %d", attempt+1, resp.StatusCode, want)
		}
		_ = resp.Body.Close()
	}
}

func TestLimiterFailsClosedWhenSharedStoreFails(t *testing.T) {
	middleware.ConfigureSharedRateLimiter(failingStore{}, nil)
	app := fiber.New()
	app.Post("/login", middleware.SensitiveRateLimit(middleware.RateLimitPolicy{Name: "unavailable", Limit: 2, Window: time.Minute}), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	resp, err := app.Test(httptest.NewRequest("POST", "/login", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", resp.StatusCode)
	}
}
