package alerts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kuipid01/salary-vault/internal/platform/mail"
)

func TestBrevoDestinationSendsOperationalEmail(t *testing.T) {
	var body, apiKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("api-key")
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	mailer := &mail.BrevoMailer{APIKey: "test-api-key", SenderName: "XAVAULT", SenderEmail: "alerts@example.test", Client: server.Client(), Endpoint: server.URL}
	destination := NewBrevoDestination(mailer, "operator@example.test")
	err := destination.Send(context.Background(), Alert{Severity: SeverityCritical, EventType: "financial_processing_halted", Title: "Financial processing halted", Message: "Invariant mismatch", Fingerprint: "halt-1", CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if apiKey != "test-api-key" {
		t.Fatal("Brevo API key header was missing")
	}
	for _, expected := range []string{"operator@example.test", "Financial processing halted", "Invariant mismatch", "operational-alert"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("Brevo payload does not contain %q: %s", expected, body)
		}
	}
}
