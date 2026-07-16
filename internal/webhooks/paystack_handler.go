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
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
)

type WebhookHandler struct {
	walletService       *wallets.WalletService
	withdrawalService   *withdrawals.WithdrawalService
	subscriptionService *subscription.SubscriptionService
}

func NewWebhookHandler(
	ws *wallets.WalletService,
	withdrawalService *withdrawals.WithdrawalService,
	subscriptionService *subscription.SubscriptionService,
) *WebhookHandler {
	return &WebhookHandler{
		walletService:       ws,
		withdrawalService:   withdrawalService,
		subscriptionService: subscriptionService,
	}
}

func (h *WebhookHandler) PaystackWebhook(c *fiber.Ctx) error {
	payload := c.Body()
	signature := c.Get("X-Paystack-Signature")

	log.Printf("[PAYSTACK_WEBHOOK][RECEIVED] payload_size=%d signature_present=%v",
		len(payload),
		signature != "",
	)

	if !isValidPaystackSignature(payload, signature) {
		log.Printf("[PAYSTACK_WEBHOOK][INVALID_SIGNATURE] payload_size=%d signature_present=%v",
			len(payload),
			signature != "",
		)

		return c.Status(fiber.StatusBadRequest).SendString("Invalid signature")
	}

	log.Printf("[PAYSTACK_WEBHOOK][SIGNATURE_VALID] payload_size=%d", len(payload))

	if err := h.ExtractAndProcessEvent(string(payload)); err != nil {
		log.Printf("[PAYSTACK_WEBHOOK][PROCESS_ERROR] error=%v payload=%s",
			err,
			string(payload),
		)

		return c.Status(fiber.StatusBadRequest).SendString("Webhook processing failed")
	}

	log.Printf("[PAYSTACK_WEBHOOK][ACKNOWLEDGED]")

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "acknowledged"})
}

func isValidPaystackSignature(payload []byte, signature string) bool {
	secret := []byte(os.Getenv("PAYSTACK_SECRET_KEY"))

	mac := hmac.New(sha512.New, secret)
	mac.Write(payload)

	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

type FlexibleBool bool

func (b *FlexibleBool) UnmarshalJSON(data []byte) error {
	var boolValue bool
	if err := json.Unmarshal(data, &boolValue); err == nil {
		*b = FlexibleBool(boolValue)
		return nil
	}

	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		switch stringValue {
		case "true", "1":
			*b = true
		case "false", "0", "":
			*b = false
		default:
			return fmt.Errorf("invalid FlexibleBool value: %q", stringValue)
		}
		return nil
	}

	return fmt.Errorf("invalid FlexibleBool value: %s", string(data))
}
func (m *ChargeSuccessMetadata) UnmarshalJSON(data []byte) error {
	if string(data) == `""` || string(data) == `null` {
		*m = ChargeSuccessMetadata{}
		return nil
	}

	type Alias ChargeSuccessMetadata
	var metadata Alias

	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}

	*m = ChargeSuccessMetadata(metadata)
	return nil
}

type ChargeSuccessMetadata struct {
	IsSubscription FlexibleBool `json:"is_subscription"`
	PlanID         string       `json:"plan_id"`
	UserID         string       `json:"user_id"`
}

func (h *WebhookHandler) ExtractAndProcessEvent(bodyText string) error {
	bodyBytes := []byte(bodyText)

	var envelope EventEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("failed to parse base envelope: %w", err)
	}

	switch envelope.Event {
	case "charge.success":
		var chargeData ChargeSuccessData
		if err := json.Unmarshal(envelope.Data, &chargeData); err != nil {
			return fmt.Errorf("failed to parse charge.success data: %w", err)
		}

		log.Printf("[PAYSTACK_WEBHOOK][CHARGE_SUCCESS] reference=%s amount=%d status=%s",
			chargeData.Reference,
			chargeData.Amount,
			chargeData.Status,
		)

		if bool(chargeData.Metadata.IsSubscription) {
			err := h.subscriptionService.ActivateSubscription(
				chargeData.Metadata.UserID,
				chargeData.Metadata.PlanID,
				chargeData.Reference,
			)
			if err != nil {
				return fmt.Errorf("failed to activate subscription: %w", err)
			}

			log.Printf("[PAYSTACK_WEBHOOK][SUBSCRIPTION_ACTIVATED] user_id=%s plan_id=%s reference=%s",
				chargeData.Metadata.UserID,
				chargeData.Metadata.PlanID,
				chargeData.Reference,
			)

			return nil
		}

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

		result := h.withdrawalService.CompleteProviderWithdrawal(
			transferData.Reference,
			transferData.Status,
		)
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
		log.Printf("[PAYSTACK_WEBHOOK][IGNORED_EVENT] event=%s", envelope.Event)
	}

	return nil
}
