package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kuipid01/salary-vault/internal/alerts"
	"github.com/kuipid01/salary-vault/internal/config"
	"github.com/kuipid01/salary-vault/internal/database"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
)

func main() {
	if os.Getenv("ALERT_TEST_CONFIRMATION") != "send" {
		fmt.Println("refusing to send; set ALERT_TEST_CONFIRMATION=send")
		os.Exit(2)
	}
	cfg := config.Load()
	if cfg.AlertEmailTo == "" {
		fmt.Println("ALERT_EMAIL_TO is required")
		os.Exit(2)
	}
	db, err := database.DB(cfg)
	if err != nil {
		fmt.Println("database connection failed:", err)
		os.Exit(1)
	}
	service := alerts.NewService(db, time.Minute, alerts.NewBrevoDestination(mail.NewBrevoMailer(), cfg.AlertEmailTo))
	fingerprint := "manual-assurance-" + time.Now().UTC().Format("20060102T150405Z")
	err = service.Publish(context.Background(), alerts.Input{
		Severity: alerts.SeverityWarning, EventType: "manual_alert_assurance",
		Title:       "XAVAULT operational alert test",
		Message:     "This is a requested assurance test. No financial incident occurred.",
		Fingerprint: fingerprint,
		Metadata:    map[string]interface{}{"source": "cmd/alerttest"},
	})
	if err == nil {
		err = service.ProcessDue(context.Background(), 1)
	}
	if err != nil {
		fmt.Println("test alert failed:", err)
		os.Exit(1)
	}
	var delivery alerts.Delivery
	if err := db.Joins("JOIN operational_alerts ON operational_alerts.id = operational_alert_deliveries.alert_id").Where("operational_alerts.fingerprint = ?", fingerprint).First(&delivery).Error; err != nil {
		fmt.Println("could not verify test delivery:", err)
		os.Exit(1)
	}
	if delivery.Status != "delivered" {
		fmt.Printf("Brevo did not accept the alert: status=%s attempts=%d error=%s\n", delivery.Status, delivery.Attempts, delivery.LastError)
		os.Exit(1)
	}
	fmt.Printf("Brevo accepted the assurance alert for %s; delivery_id=%s\n", cfg.AlertEmailTo, delivery.ID)
}
