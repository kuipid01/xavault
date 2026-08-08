//go:build integration

package webhooks_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	client "github.com/kuipid01/salary-vault/internal/shared/client/idempotence"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/webhooks"
)

func TestSignedChargeWebhookPersistsAndCreditsExactlyOnce(t *testing.T) {
	db := testsupport.FinancialDB(t)
	user, wallet := testsupport.SeedUserWallet(t, db, 0, 0, 0)
	testsupport.SeedActiveSubscription(t, db, user.ID)
	reference := "WEBHOOK-DEPOSIT-" + uuid.NewString()
	deposit := wallets.WalletDeposit{
		ID: uuid.NewString(), UserID: user.ID, WalletID: wallet.ID, Reference: reference,
		Amount: 7500, Currency: "NGN", Status: "pending", Provider: "paystack",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	walletRepo := wallets.NewWalletRepository(db)
	if _, err := walletRepo.CreateDeposit(&deposit); err != nil {
		t.Fatal(err)
	}
	walletService := wallets.NewWalletService(walletRepo, nil, client.NewIdempotentRequest(client.NewIdempotentRepository(db), true), subscription.NewSubscriptionRepository(db))
	handler := webhooks.NewWebhookHandler(db, walletService, nil, nil)
	app := fiber.New()
	app.Post("/paystack", handler.PaystackWebhook)

	secret := "phase-three-webhook-secret"
	t.Setenv("PAYSTACK_SECRET_KEY", secret)
	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"id":883721,"amount":7500,"reference":%q,"status":"success","currency":"NGN","metadata":{"is_subscription":false}}}`, reference))
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	for attempt := 0; attempt < 2; attempt++ {
		req := httptest.NewRequest("POST", "/paystack", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Paystack-Signature", signature)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("webhook attempt %d returned %d", attempt+1, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	updated, err := walletRepo.GetWalletByID(wallet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AvailableBalance != 7500 || updated.LedgerBalance != 7500 {
		t.Fatalf("webhook did not allocate deposit exactly once: %+v", updated)
	}
	var ledgerCount, eventCount int64
	if err := db.Model(&wallets.WalletLedger{}).Where("reference = ?", reference).Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&webhooks.PaystackEvent{}).Where("resource_reference = ? AND status = ?", reference, webhooks.EventStatusProcessed).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || eventCount != 1 {
		t.Fatalf("got ledgers=%d processed_events=%d, want one each", ledgerCount, eventCount)
	}
}
