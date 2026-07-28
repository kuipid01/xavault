package webhooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	db *gorm.DB
}

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

// Claim moves a due event into processing while holding a row lock.
func (s *EventStore) Claim(id string) (*PaystackEvent, bool, error) {
	var claimed *PaystackEvent
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var event PaystackEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&event).Error; err != nil {
			return err
		}
		now := time.Now()
		if event.Status == EventStatusProcessed || event.Status == EventStatusDead {
			return nil
		}
		if event.Status == EventStatusProcessing &&
			event.ProcessingStartedAt != nil &&
			event.ProcessingStartedAt.After(now.Add(-5*time.Minute)) {
			return nil
		}
		if event.NextRetryAt != nil && event.NextRetryAt.After(now) {
			return nil
		}
		event.Status = EventStatusProcessing
		event.Attempts++
		event.ProcessingStartedAt = &now
		event.NextRetryAt = nil
		if err := tx.Save(&event).Error; err != nil {
			return err
		}
		claimed = &event
		return nil
	})
	return claimed, claimed != nil, err
}

func (s *EventStore) MarkProcessed(id string) error {
	now := time.Now()
	return s.db.Model(&PaystackEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":                EventStatusProcessed,
		"processed_at":          &now,
		"processing_started_at": nil,
		"next_retry_at":         nil,
		"last_error":            "",
	}).Error
}

func (s *EventStore) MarkFailed(event *PaystackEvent, processingErr error) error {
	status := EventStatusRetrying
	var retryAt *time.Time
	if event.Attempts >= maxEventAttempts {
		status = EventStatusDead
	} else {
		next := time.Now().Add(eventRetryDelay(event.Attempts))
		retryAt = &next
	}
	message := "unknown processing error"
	if processingErr != nil {
		message = processingErr.Error()
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	return s.db.Model(&PaystackEvent{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"status":                status,
		"last_error":            message,
		"next_retry_at":         retryAt,
		"processing_started_at": nil,
	}).Error
}

func (s *EventStore) Due(limit int) ([]PaystackEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var events []PaystackEvent
	err := s.db.
		Where(
			"(status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR "+
				"(status = ? AND processing_started_at < ?)",
			[]string{EventStatusPending, EventStatusRetrying},
			time.Now(),
			EventStatusProcessing,
			time.Now().Add(-5*time.Minute),
		).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
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
