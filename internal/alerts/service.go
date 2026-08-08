package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

type Alert struct {
	ID           string `gorm:"type:uuid;primaryKey"`
	Severity     string
	EventType    string
	Title        string
	Message      string
	Fingerprint  string
	DedupeKey    string
	MetadataJSON string `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (Alert) TableName() string { return "operational_alerts" }

type Delivery struct {
	ID             string `gorm:"type:uuid;primaryKey"`
	AlertID        string `gorm:"type:uuid"`
	Destination    string
	Status         string
	Attempts       int
	NextAttemptAt  *time.Time
	LastError      string
	DeliveredAt    *time.Time
	LeaseToken     string
	LeaseExpiresAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Delivery) TableName() string { return "operational_alert_deliveries" }

type Input struct {
	Severity    string
	EventType   string
	Title       string
	Message     string
	Fingerprint string
	Metadata    map[string]interface{}
}

type Destination interface {
	Name() string
	Send(context.Context, Alert) error
}

type Service struct {
	db           *gorm.DB
	destinations map[string]Destination
	cooldown     time.Duration
}

func NewService(db *gorm.DB, cooldown time.Duration, destinations ...Destination) *Service {
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	registered := make(map[string]Destination, len(destinations))
	for _, destination := range destinations {
		if destination != nil {
			registered[destination.Name()] = destination
		}
	}
	return &Service{db: db, destinations: registered, cooldown: cooldown}
}

func (s *Service) Publish(ctx context.Context, input Input) error {
	if s == nil || s.db == nil || len(s.destinations) == 0 {
		return nil
	}
	input.EventType = strings.TrimSpace(input.EventType)
	input.Fingerprint = strings.TrimSpace(input.Fingerprint)
	if input.Fingerprint == "" {
		input.Fingerprint = input.EventType
	}
	metadata, err := json.Marshal(sanitizeMetadata(input.Metadata))
	if err != nil {
		return fmt.Errorf("encode alert metadata: %w", err)
	}
	bucket := time.Now().UTC().Unix() / int64(s.cooldown.Seconds())
	alert := Alert{ID: uuid.NewString(), Severity: input.Severity, EventType: input.EventType, Title: input.Title, Message: input.Message, Fingerprint: input.Fingerprint, DedupeKey: fmt.Sprintf("%s:%d", input.Fingerprint, bucket), MetadataJSON: string(metadata), CreatedAt: time.Now().UTC()}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedupe_key"}}, DoNothing: true}).Create(&alert)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		for name := range s.destinations {
			delivery := Delivery{ID: uuid.NewString(), AlertID: alert.ID, Destination: name, Status: "pending", CreatedAt: alert.CreatedAt, UpdatedAt: alert.CreatedAt}
			if err := tx.Create(&delivery).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ProcessDue(ctx context.Context, limit int) error {
	if s == nil || s.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 25
	}
	for i := 0; i < limit; i++ {
		var delivery Delivery
		lease := uuid.NewString()
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_expires_at IS NULL OR lease_expires_at < ?)", []string{"pending", "retrying"}, now, now).Order("created_at ASC").Limit(1).Find(&delivery)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			expires := now.Add(2 * time.Minute)
			return tx.Model(&delivery).Updates(map[string]interface{}{"lease_token": lease, "lease_expires_at": expires}).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var alert Alert
		if err := s.db.WithContext(ctx).First(&alert, "id = ?", delivery.AlertID).Error; err != nil {
			return err
		}
		destination := s.destinations[delivery.Destination]
		var sendErr error
		if destination == nil {
			sendErr = fmt.Errorf("alert destination %q is not configured", delivery.Destination)
		} else {
			sendErr = destination.Send(ctx, alert)
		}
		now := time.Now().UTC()
		if sendErr == nil {
			if err := s.db.Model(&Delivery{}).Where("id = ? AND lease_token = ?", delivery.ID, lease).Updates(map[string]interface{}{"status": "delivered", "attempts": delivery.Attempts + 1, "delivered_at": now, "last_error": "", "lease_token": "", "lease_expires_at": nil}).Error; err != nil {
				return err
			}
			continue
		}
		attempts := delivery.Attempts + 1
		status := "retrying"
		if attempts >= 8 {
			status = "dead"
		}
		next := now.Add(retryDelay(attempts))
		if err := s.db.Model(&Delivery{}).Where("id = ? AND lease_token = ?", delivery.ID, lease).Updates(map[string]interface{}{"status": status, "attempts": attempts, "next_attempt_at": next, "last_error": truncate(sendErr.Error(), 1000), "lease_token": "", "lease_expires_at": nil}).Error; err != nil {
			return err
		}
		log.Printf(`{"level":"error","event":"operational_alert_delivery_failed","destination":%q,"attempt":%d}`, delivery.Destination, attempts)
	}
	return nil
}

func (s *Service) StartWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		if err := s.ProcessDue(ctx, 25); err != nil {
			log.Printf(`{"level":"error","event":"alert_worker_failed","error":%q}`, err.Error())
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.ProcessDue(ctx, 25); err != nil {
					log.Printf(`{"level":"error","event":"alert_worker_failed","error":%q}`, err.Error())
				}
			}
		}
	}()
}

func retryDelay(attempt int) time.Duration {
	delays := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour, 6 * time.Hour}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(delays) {
		attempt = len(delays)
	}
	return delays[attempt-1]
}
func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
func sanitizeMetadata(values map[string]interface{}) map[string]interface{} {
	clean := map[string]interface{}{}
	for key, value := range values {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "pin") || strings.Contains(lower, "payload") {
			clean[key] = "[REDACTED]"
		} else {
			clean[key] = value
		}
	}
	return clean
}
