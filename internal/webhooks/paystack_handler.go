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
	"github.com/kuipid01/salary-vault/internal/withdrawals"
)

type WebhookHandler struct {
	walletService     *wallets.WalletService
	withdrawalService *withdrawals.WithdrawalService
}

func NewWebhookHandler(ws *wallets.WalletService, withdrawalService *withdrawals.WithdrawalService) *WebhookHandler {
	return &WebhookHandler{walletService: ws, withdrawalService: withdrawalService}
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
		if chargeData.DedicatedAccount.AccountNumber != "" {
			h.walletService.CreditDedicatedAccountTransfer(
				chargeData.Reference,
				chargeData.Amount,
				chargeData.DedicatedAccount.AccountNumber,
			)
		} else {
			h.walletService.CreditWallet(chargeData.Reference, chargeData.Amount)
		}

	case "transfer.success":
		var transferData TransferEventData
		if err := json.Unmarshal(envelope.Data, &transferData); err != nil {
			return fmt.Errorf("failed to parse transfer.success data: %w", err)
		}

		result := h.withdrawalService.CompleteProviderWithdrawal(transferData.Reference, transferData.Status)
		if result.Error != nil {
			return fmt.Errorf("failed to complete withdrawal: %v", result.Error)
		}

	case "transfer.failed", "transfer.reversed":
		var transferData TransferEventData
		if err := json.Unmarshal(envelope.Data, &transferData); err != nil {
			return fmt.Errorf("failed to parse %s data: %w", envelope.Event, err)
		}

		result := h.withdrawalService.FailProviderWithdrawal(
			transferData.Reference,
			transferData.Status,
			transferData.Reason,
		)
		if result.Error != nil {
			return fmt.Errorf("failed to fail withdrawal: %v", result.Error)
		}
	default:
		log.Printf("ℹ️ Ignored unhandled event hook: %s", envelope.Event)
	}

	return nil
}
