package webhooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/alerts"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	EventStatusPending    = "pending"
	EventStatusProcessing = "processing"
	EventStatusProcessed  = "processed"
	EventStatusRetrying   = "retrying"
	EventStatusDead       = "dead"
)

const maxEventAttempts = 10

// PaystackEvent is the durable inbox for provider notifications. The unique
// provider_event_id is the concurrency boundary that prevents duplicate money
// movement across webhook retries and multiple API instances.
type PaystackEvent struct {
	ID                  string     `gorm:"type:uuid;primaryKey" json:"id"`
	ProviderEventID     string     `gorm:"type:varchar(220);not null;uniqueIndex" json:"provider_event_id"`
	EventType           string     `gorm:"type:varchar(80);not null;index" json:"event_type"`
	ResourceID          int64      `gorm:"index" json:"resource_id"`
	ResourceReference   string     `gorm:"type:varchar(140);index" json:"resource_reference"`
	PayloadHash         string     `gorm:"type:char(64);not null" json:"payload_hash"`
	Payload             string     `gorm:"type:text;not null" json:"-"`
	Status              string     `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	Attempts            int        `gorm:"not null;default:0" json:"attempts"`
	LastError           string     `gorm:"type:text" json:"last_error,omitempty"`
	NextRetryAt         *time.Time `gorm:"index" json:"next_retry_at,omitempty"`
	ProcessingStartedAt *time.Time `json:"processing_started_at,omitempty"`
	LeaseExpiresAt      *time.Time `gorm:"index" json:"lease_expires_at,omitempty"`
	LeaseToken          string     `gorm:"type:varchar(64);index" json:"-"`
	ProcessedAt         *time.Time `json:"processed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (e *PaystackEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Status == "" {
		e.Status = EventStatusPending
	}
	return nil
}

type EventStore struct {
	db     *gorm.DB
	alerts *alerts.Service
}

func (s *EventStore) SetAlertPublisher(publisher *alerts.Service) { s.alerts = publisher }

func NewEventStore(db *gorm.DB) *EventStore {
	return &EventStore{db: db}
}

func NewPaystackEvent(payload []byte) (*PaystackEvent, error) {
	var envelope struct {
		Event string `json:"event"`
		Data  struct {
			ID        int64  `json:"id"`
			Reference string `json:"reference"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("invalid Paystack event envelope: %w", err)
	}
	envelope.Event = strings.TrimSpace(envelope.Event)
	envelope.Data.Reference = strings.TrimSpace(envelope.Data.Reference)
	if envelope.Event == "" {
		return nil, errors.New("Paystack event type is required")
	}

	sum := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(sum[:])
	providerID := ""
	if envelope.Data.ID != 0 {
		providerID = fmt.Sprintf("paystack:%s:%d", envelope.Event, envelope.Data.ID)
	} else if envelope.Data.Reference != "" {
		// The event type is included because a successful transfer can later
		// receive a legitimate reversal event for the same reference.
		providerID = fmt.Sprintf(
			"paystack:%s:%s:%s",
			envelope.Event,
			envelope.Data.Reference,
			strings.ToLower(strings.TrimSpace(envelope.Data.Status)),
		)
	} else {
		// Retain malformed/less common events for audit and stable deduplication.
		providerID = "paystack:" + envelope.Event + ":" + payloadHash
	}

	return &PaystackEvent{
		ProviderEventID:   providerID,
		EventType:         envelope.Event,
		ResourceID:        envelope.Data.ID,
		ResourceReference: envelope.Data.Reference,
		PayloadHash:       payloadHash,
		Payload:           string(payload),
	}, nil
}

// Receive stores an event once. inserted=false means Paystack redelivered an
// event already present in the durable inbox.
func (s *EventStore) Receive(event *PaystackEvent) (inserted bool, err error) {
	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_event_id"}},
		DoNothing: true,
	}).Create(event)
	return result.RowsAffected == 1, result.Error
}

// Claim moves one explicitly selected event into processing. Recovery workers
// use ClaimDueBatch so selection and claiming remain one atomic operation.
func (s *EventStore) Claim(id string) (*PaystackEvent, bool, error) {
	var claimed *PaystackEvent
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var event PaystackEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&event).Error; err != nil {
			return err
		}
		if event.Status == EventStatusProcessed || event.Status == EventStatusDead {
			return nil
		}
		var leaseActive bool
		if event.Status == EventStatusProcessing {
			if err := tx.Raw("SELECT lease_expires_at IS NOT NULL AND lease_expires_at > NOW() FROM paystack_events WHERE id = ?", id).
				Scan(&leaseActive).Error; err != nil {
				return err
			}
		}
		if leaseActive {
			return nil
		}
		var retryNotDue bool
		if event.NextRetryAt != nil {
			if err := tx.Raw("SELECT next_retry_at > NOW() FROM paystack_events WHERE id = ?", id).
				Scan(&retryNotDue).Error; err != nil {
				return err
			}
		}
		if retryNotDue {
			return nil
		}
		leaseToken := uuid.NewString()
		if err := tx.Model(&PaystackEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":                EventStatusProcessing,
			"attempts":              gorm.Expr("attempts + 1"),
			"processing_started_at": gorm.Expr("NOW()"),
			"lease_expires_at":      gorm.Expr("NOW() + INTERVAL '5 minutes'"),
			"lease_token":           leaseToken,
			"next_retry_at":         nil,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).First(&event).Error; err != nil {
			return err
		}
		claimed = &event
		return nil
	})
	return claimed, claimed != nil, err
}

// ClaimDueBatch atomically selects and claims different rows across instances.
// SKIP LOCKED prevents workers on other nodes from waiting on the same batch.
func (s *EventStore) ClaimDueBatch(limit int) ([]PaystackEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var claimed []PaystackEvent
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var candidates []PaystackEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				"(status IN ? AND (next_retry_at IS NULL OR next_retry_at <= NOW())) OR "+
					"(status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= NOW()))",
				[]string{EventStatusPending, EventStatusRetrying},
				EventStatusProcessing,
			).
			Order("created_at ASC").
			Limit(limit).
			Find(&candidates).Error; err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		ids := make([]string, 0, len(candidates))
		for i := range candidates {
			ids = append(ids, candidates[i].ID)
		}
		leaseToken := uuid.NewString()
		if err := tx.Model(&PaystackEvent{}).Where("id IN ?", ids).Updates(map[string]interface{}{
			"status":                EventStatusProcessing,
			"attempts":              gorm.Expr("attempts + 1"),
			"processing_started_at": gorm.Expr("NOW()"),
			"lease_expires_at":      gorm.Expr("NOW() + INTERVAL '5 minutes'"),
			"lease_token":           leaseToken,
			"next_retry_at":         nil,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Order("created_at ASC").Find(&claimed).Error
	})
	return claimed, err
}

func (s *EventStore) Heartbeat(id, leaseToken string) error {
	result := s.db.Model(&PaystackEvent{}).
		Where("id = ? AND status = ? AND lease_token = ?", id, EventStatusProcessing, leaseToken).
		Update("lease_expires_at", gorm.Expr("NOW() + INTERVAL '5 minutes'"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("event lease is no longer owned")
	}
	return nil
}

// CompleteInTransaction commits the financial mutation and inbox completion
// together. A stale worker cannot commit after its lease has been reassigned.
func (s *EventStore) CompleteInTransaction(event *PaystackEvent, fn func(tx *gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var current PaystackEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", event.ID).First(&current).Error; err != nil {
			return err
		}
		if current.Status != EventStatusProcessing || current.LeaseToken == "" || current.LeaseToken != event.LeaseToken {
			return errors.New("event processing lease was lost")
		}
		if err := fn(tx); err != nil {
			return err
		}
		result := tx.Model(&PaystackEvent{}).
			Where("id = ? AND status = ? AND lease_token = ?", event.ID, EventStatusProcessing, event.LeaseToken).
			Updates(map[string]interface{}{
				"status":                EventStatusProcessed,
				"processed_at":          gorm.Expr("NOW()"),
				"processing_started_at": nil,
				"lease_expires_at":      nil,
				"lease_token":           "",
				"next_retry_at":         nil,
				"last_error":            "",
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("event processing lease was lost before completion")
		}
		return nil
	})
}

func (s *EventStore) MarkFailed(event *PaystackEvent, processingErr error) error {
	status := EventStatusRetrying
	var retryAt interface{}
	if event.Attempts >= maxEventAttempts {
		status = EventStatusDead
	} else {
		seconds := int64(eventRetryDelay(event.Attempts).Seconds())
		retryAt = gorm.Expr("NOW() + (? * INTERVAL '1 second')", seconds)
	}
	message := "unknown processing error"
	if processingErr != nil {
		message = processingErr.Error()
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	result := s.db.Model(&PaystackEvent{}).
		Where("id = ? AND status = ? AND lease_token = ?", event.ID, EventStatusProcessing, event.LeaseToken).
		Updates(map[string]interface{}{
			"status":                status,
			"last_error":            message,
			"next_retry_at":         retryAt,
			"processing_started_at": nil,
			"lease_expires_at":      nil,
			"lease_token":           "",
		})
	if result.Error == nil && status == EventStatusDead && s.alerts != nil {
		if err := s.alerts.Publish(context.Background(), alerts.Input{Severity: alerts.SeverityCritical, EventType: "webhook_dead", Title: "Webhook exhausted all retries", Message: "A durable Paystack event could not be processed", Fingerprint: "webhook-dead:" + event.ProviderEventID, Metadata: map[string]interface{}{"event_id": event.ProviderEventID, "event_type": event.EventType, "reference": event.ResourceReference, "error": message}}); err != nil {
			log.Printf(`{"level":"error","event":"operational_alert_enqueue_failed","alert_type":"webhook_dead","error":%q}`, err.Error())
		}
	}
	return result.Error
}

func (s *EventStore) FinancialProcessingHalted() (bool, error) {
	var row struct {
		ProcessingHalted bool
	}
	err := s.db.Table("reconciliation_controls").
		Select("processing_halted").
		Where("id = ?", 1).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return row.ProcessingHalted, err
}

func (s *EventStore) MakeDue(id string) error {
	return s.db.Model(&PaystackEvent{}).
		Where("id = ? AND status <> ?", id, EventStatusProcessed).
		Updates(map[string]interface{}{
			"status":           EventStatusRetrying,
			"next_retry_at":    nil,
			"last_error":       "",
			"lease_expires_at": nil,
			"lease_token":      "",
		}).Error
}

func eventRetryDelay(attempt int) time.Duration {
	delays := []time.Duration{
		time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		time.Hour,
		3 * time.Hour,
		6 * time.Hour,
	}
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return delays[index]
}
