package webhooks

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/wallets"
)

type WebhookHandler struct {
	walletService *wallets.WalletService
}

func NewWebhookHandler(ws *wallets.WalletService) *WebhookHandler {
	return &WebhookHandler{walletService: ws}
}

// PaystackWebhook handles all events from Paystack in one place
func (h *WebhookHandler) PaystackWebhook(c *fiber.Ctx) error {
	payload := c.Body()
	signature := c.Get("X-Paystack-Signature")

	// 1. Enforce strict signature validation right here
	if !isValidPaystackSignature(payload, signature) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid signature")
	}
	err := h.ExtractAndProcessEvent(string(payload))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid signature")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "acknowledged"})
}

func isValidPaystackSignature(payload []byte, signature string) bool {
	secret := []byte(os.Getenv("PAYSTACK_SECRET_KEY"))
	hmac := hmac.New(sha512.New, secret)
	hmac.Write(payload)
	signedPayload := hex.EncodeToString(hmac.Sum(nil))

	return signedPayload == signature
}

// ExtractAndProcessEvent parses raw body text into typed domain models
func (h *WebhookHandler) ExtractAndProcessEvent(bodyText string) error {
	// 1. Convert text stream into raw byte segment
	bodyBytes := []byte(bodyText)

	var envelope EventEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("failed to parse base envelope: %w", err)
	}

	// 2. Evaluate the event type and run specific parser targets
	switch envelope.Event {
	case "charge.success":
		var chargeData ChargeSuccessData
		// Unmarshal directly from the preserved raw message bytes
		if err := json.Unmarshal(envelope.Data, &chargeData); err != nil {
			return fmt.Errorf("failed to parse charge.success data: %w", err)
		}

		// Hand off to your business handlers smoothly
		fmt.Printf("🎉 Deposit verified! Ref: %s, Amount: %d kobo", chargeData.Reference, chargeData.Amount)
		h.walletService.CreditWallet(chargeData.Reference, chargeData.Amount)

	case "transfer.reversed":
		// Handle transfer reversals here...
		fmt.Printf("⚠️ Transfer reversed event detected")

	default:
		log.Printf("ℹ️ Ignored unhandled event hook: %s", envelope.Event)
	}

	return nil
}
