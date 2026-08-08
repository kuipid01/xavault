//go:build integration

package alerts_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kuipid01/salary-vault/internal/alerts"
	"github.com/kuipid01/salary-vault/internal/testsupport"
)

type fakeDestination struct {
	name     string
	calls    int
	err      error
	received alerts.Alert
}

func (f *fakeDestination) Name() string { return f.name }
func (f *fakeDestination) Send(_ context.Context, alert alerts.Alert) error {
	f.calls++
	f.received = alert
	return f.err
}

func TestAlertOutboxDeduplicatesAndDelivers(t *testing.T) {
	db := testsupport.FinancialDB(t)
	destination := &fakeDestination{name: "fake"}
	service := alerts.NewService(db, 15*time.Minute, destination)
	input := alerts.Input{Severity: alerts.SeverityCritical, EventType: "financial_processing_halted", Title: "Financial processing halted", Message: "test invariant failure", Fingerprint: "halt", Metadata: map[string]interface{}{"reference": "RUN-1", "access_token": "must-not-leak"}}
	if err := service.Publish(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := service.ProcessDue(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if destination.calls != 1 {
		t.Fatalf("got %d sends, want 1", destination.calls)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(destination.received.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["access_token"] != "[REDACTED]" || metadata["reference"] != "RUN-1" {
		t.Fatalf("unexpected sanitized metadata %v", metadata)
	}
	var alertsCount, delivered int64
	if err := db.Model(&alerts.Alert{}).Count(&alertsCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&alerts.Delivery{}).Where("status = ?", "delivered").Count(&delivered).Error; err != nil {
		t.Fatal(err)
	}
	if alertsCount != 1 || delivered != 1 {
		t.Fatalf("alerts=%d delivered=%d", alertsCount, delivered)
	}
}

func TestAlertFailureIsRetriedWithoutLosingOutboxRecord(t *testing.T) {
	db := testsupport.FinancialDB(t)
	destination := &fakeDestination{name: "failing", err: errors.New("mail unavailable")}
	service := alerts.NewService(db, time.Minute, destination)
	if err := service.Publish(context.Background(), alerts.Input{Severity: alerts.SeverityError, EventType: "webhook_dead", Title: "Webhook failed", Message: "retry me", Fingerprint: "event-1"}); err != nil {
		t.Fatal(err)
	}
	if err := service.ProcessDue(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var delivery alerts.Delivery
	if err := db.First(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != "retrying" || delivery.Attempts != 1 || delivery.NextAttemptAt == nil {
		t.Fatalf("unexpected retry state: %+v", delivery)
	}
}
