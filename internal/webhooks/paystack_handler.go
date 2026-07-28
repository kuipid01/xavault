package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/wallets"
	"github.com/kuipid01/salary-vault/internal/withdrawals"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	walletService       *wallets.WalletService
	withdrawalService   *withdrawals.WithdrawalService
	subscriptionService *subscription.SubscriptionService
	eventStore          *EventStore
}

func NewWebhookHandler(
	db *gorm.DB,
	ws *wallets.WalletService,
	withdrawalService *withdrawals.WithdrawalService,
	subscriptionService *subscription.SubscriptionService,
) *WebhookHandler {
	return &WebhookHandler{
		walletService:       ws,
		withdrawalService:   withdrawalService,
		subscriptionService: subscriptionService,
		eventStore:          NewEventStore(db),
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

	event, err := NewPaystackEvent(payload)
	if err != nil {
		log.Printf("[PAYSTACK_WEBHOOK][INVALID_EVENT] error=%v", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid webhook payload")
	}
	inserted, err := h.eventStore.Receive(event)
	if err != nil {
		log.Printf("[PAYSTACK_WEBHOOK][PERSIST_ERROR] event_id=%s error=%v", event.ProviderEventID, err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("Webhook persistence failed")
	}
	if !inserted {
		log.Printf("[PAYSTACK_WEBHOOK][DUPLICATE] event_id=%s", event.ProviderEventID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "already_received"})
	}
	if err := h.processStoredEvent(event.ID); err != nil {
		// The payload is durable; acknowledge it and let the recovery worker
		// retry so provider redelivery is not the only recovery mechanism.
		log.Printf("[PAYSTACK_WEBHOOK][PROCESS_DEFERRED] event_id=%s error=%v", event.ProviderEventID, err)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "accepted_for_retry"})
	}

	log.Printf("[PAYSTACK_WEBHOOK][ACKNOWLEDGED]")

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "acknowledged"})
}

func (h *WebhookHandler) processStoredEvent(id string) error {
	event, claimed, err := h.eventStore.Claim(id)
	if err != nil || !claimed {
		return err
	}
	if err := h.ExtractAndProcessEvent(event.Payload); err != nil {
		if markErr := h.eventStore.MarkFailed(event, err); markErr != nil {
			return fmt.Errorf("process error: %v; failed to persist retry: %w", err, markErr)
		}
		return err
	}
	return h.eventStore.MarkProcessed(event.ID)
}

// StartRecoveryWorker retries durable webhook events. Stop it by cancelling ctx.
func (h *WebhookHandler) StartRecoveryWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := h.eventStore.Due(50)
				if err != nil {
					log.Printf("[PAYSTACK_WEBHOOK][RECOVERY_QUERY_ERROR] error=%v", err)
					continue
				}
				for i := range events {
					if err := h.processStoredEvent(events[i].ID); err != nil {
						log.Printf("[PAYSTACK_WEBHOOK][RECOVERY_RETRY_FAILED] event_id=%s error=%v", events[i].ProviderEventID, err)
					}
				}
			}
		}
	}()
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

	log.Printf("[PAYSTACK_WEBHOOK][EVENT] event=%s", envelope.Event)

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
				chargeData.Amount,
				chargeData.Currency,
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
			result := h.walletService.CreditDedicatedAccountTransfer(
				chargeData.Reference,
				chargeData.Amount,
				chargeData.DedicatedAccount.AccountNumber,
			)
			if result.Error != nil {
				return fmt.Errorf("failed to credit dedicated account deposit: %v", result.Error)
			}
		} else {
			result := h.walletService.CreditWallet(chargeData.Reference, chargeData.Amount)
			if result.Error != nil {
				return fmt.Errorf("failed to credit wallet deposit: %v", result.Error)
			}
		}

	case "transfer.success":
		var transferData TransferEventData
		if err := json.Unmarshal(envelope.Data, &transferData); err != nil {
			return fmt.Errorf("failed to parse transfer.success data: %w", err)
		}

		log.Printf("[PAYSTACK_WEBHOOK][TRANSFER_SUCCESS] reference=%s status=%s",
			transferData.Reference,
			transferData.Status,
		)

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

		log.Printf("[PAYSTACK_WEBHOOK][TRANSFER_FAILED] event=%s reference=%s status=%s reason=%s",
			envelope.Event,
			transferData.Reference,
			transferData.Status,
			transferData.Reason,
		)

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
