package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/alerts"
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

func (h *WebhookHandler) SetAlertPublisher(publisher *alerts.Service) {
	h.eventStore.SetAlertPublisher(publisher)
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
	halted, haltErr := h.eventStore.FinancialProcessingHalted()
	if haltErr != nil {
		log.Printf("[PAYSTACK_WEBHOOK][SAFETY_CHECK_ERROR] event_id=%s error=%v", event.ProviderEventID, haltErr)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "accepted_for_retry"})
	}
	if halted {
		log.Printf("[PAYSTACK_WEBHOOK][DEFERRED_BY_SAFETY_HALT] event_id=%s", event.ProviderEventID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "accepted_while_processing_halted"})
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
	halted, err := h.eventStore.FinancialProcessingHalted()
	if err != nil {
		return err
	}
	if halted {
		return nil
	}
	event, claimed, err := h.eventStore.Claim(id)
	if err != nil || !claimed {
		return err
	}
	return h.processClaimedEvent(event)
}

func (h *WebhookHandler) processClaimedEvent(event *PaystackEvent) error {
	heartbeatCtx, stopHeartbeat := context.WithCancel(context.Background())
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := h.eventStore.Heartbeat(event.ID, event.LeaseToken); err != nil {
					log.Printf("[PAYSTACK_WEBHOOK][LEASE_HEARTBEAT_FAILED] event_id=%s error=%v", event.ProviderEventID, err)
					return
				}
			}
		}
	}()
	defer func() {
		stopHeartbeat()
		<-heartbeatDone
	}()

	if err := h.eventStore.CompleteInTransaction(event, func(tx *gorm.DB) error {
		return h.ExtractAndProcessEventTx(tx, event.Payload)
	}); err != nil {
		if markErr := h.eventStore.MarkFailed(event, err); markErr != nil {
			return fmt.Errorf("process error: %v; failed to persist retry: %w", err, markErr)
		}
		return err
	}
	return nil
}

// StartRecoveryWorker retries durable webhook events. Stop it by cancelling ctx.
func (h *WebhookHandler) StartRecoveryWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	const workerCount = 5
	const batchSize = 25
	jobs := make(chan PaystackEvent, batchSize)
	for workerID := 0; workerID < workerCount; workerID++ {
		go func(id int) {
			for {
				select {
				case <-ctx.Done():
					return
				case event, ok := <-jobs:
					if !ok {
						return
					}
					if err := h.processClaimedEvent(&event); err != nil {
						log.Printf("[PAYSTACK_WEBHOOK][RECOVERY_RETRY_FAILED] worker=%d event_id=%s error=%v", id, event.ProviderEventID, err)
					}
				}
			}
		}(workerID)
	}
	go func() {
		defer close(jobs)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		dispatch := func() {
			halted, err := h.eventStore.FinancialProcessingHalted()
			if err != nil {
				log.Printf("[PAYSTACK_WEBHOOK][RECOVERY_SAFETY_CHECK_ERROR] error=%v", err)
				return
			}
			if halted {
				return
			}
			events, err := h.eventStore.ClaimDueBatch(batchSize)
			if err != nil {
				log.Printf("[PAYSTACK_WEBHOOK][RECOVERY_CLAIM_ERROR] error=%v", err)
				return
			}
			for i := range events {
				select {
				case jobs <- events[i]:
				case <-ctx.Done():
					return
				}
			}
		}
		dispatch()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dispatch()
			}
		}
	}()
}

func (h *WebhookHandler) ReplayStoredEvent(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "event_id_required"})
	}
	if err := h.eventStore.MakeDue(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "event_replay_failed"})
	}
	if err := h.processStoredEvent(id); err != nil {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "scheduled_for_retry"})
	}
	return c.JSON(fiber.Map{"status": "replayed"})
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
	return h.extractAndProcessEvent(nil, bodyText)
}

func (h *WebhookHandler) ExtractAndProcessEventTx(tx *gorm.DB, bodyText string) error {
	if tx == nil {
		return errors.New("webhook transaction is required")
	}
	return h.extractAndProcessEvent(tx, bodyText)
}

func (h *WebhookHandler) extractAndProcessEvent(tx *gorm.DB, bodyText string) error {
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
			var err error
			if tx != nil {
				err = h.subscriptionService.ActivateSubscriptionTx(
					tx, chargeData.Metadata.UserID, chargeData.Metadata.PlanID,
					chargeData.Reference, chargeData.Amount, chargeData.Currency,
				)
			} else {
				err = h.subscriptionService.ActivateSubscription(
					chargeData.Metadata.UserID, chargeData.Metadata.PlanID,
					chargeData.Reference, chargeData.Amount, chargeData.Currency,
				)
			}
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
			if tx != nil {
				if _, err := h.walletService.CreditDedicatedAccountTransferWithTx(
					tx, chargeData.Reference, chargeData.Amount, chargeData.DedicatedAccount.AccountNumber,
				); err != nil {
					return fmt.Errorf("failed to credit dedicated account deposit: %w", err)
				}
			} else {
				result := h.walletService.CreditDedicatedAccountTransfer(
					chargeData.Reference, chargeData.Amount, chargeData.DedicatedAccount.AccountNumber,
				)
				if result.Error != nil {
					return fmt.Errorf("failed to credit dedicated account deposit: %v", result.Error)
				}
			}
		} else {
			if tx != nil {
				if _, err := h.walletService.CreditWalletWithTx(tx, chargeData.Reference, chargeData.Amount); err != nil {
					return fmt.Errorf("failed to credit wallet deposit: %w", err)
				}
			} else {
				result := h.walletService.CreditWallet(chargeData.Reference, chargeData.Amount)
				if result.Error != nil {
					return fmt.Errorf("failed to credit wallet deposit: %v", result.Error)
				}
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

		if tx != nil {
			if err := h.withdrawalService.CompleteProviderWithdrawalTx(tx, transferData.Reference, transferData.Status); err != nil {
				return fmt.Errorf("failed to complete withdrawal: %w", err)
			}
		} else {
			result := h.withdrawalService.CompleteProviderWithdrawal(transferData.Reference, transferData.Status)
			if result.Error != nil {
				return fmt.Errorf("failed to complete withdrawal: %v", result.Error)
			}
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

		if tx != nil {
			if err := h.withdrawalService.FailProviderWithdrawalTx(
				tx, transferData.Reference, transferData.Status, transferData.Reason,
			); err != nil {
				return fmt.Errorf("failed to fail withdrawal: %w", err)
			}
		} else {
			result := h.withdrawalService.FailProviderWithdrawal(
				transferData.Reference, transferData.Status, transferData.Reason,
			)
			if result.Error != nil {
				return fmt.Errorf("failed to fail withdrawal: %v", result.Error)
			}
		}

	default:
		log.Printf("[PAYSTACK_WEBHOOK][IGNORED_EVENT] event=%s", envelope.Event)
	}

	return nil
}
