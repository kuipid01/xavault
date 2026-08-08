package alerts

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/kuipid01/salary-vault/internal/platform/mail"
)

type BrevoDestination struct {
	mailer    *mail.BrevoMailer
	recipient string
}

func NewBrevoDestination(mailer *mail.BrevoMailer, recipient string) *BrevoDestination {
	return &BrevoDestination{mailer: mailer, recipient: strings.TrimSpace(recipient)}
}

func (b *BrevoDestination) Name() string { return "brevo_email" }

func (b *BrevoDestination) Send(ctx context.Context, alert Alert) error {
	if b == nil || b.mailer == nil || b.recipient == "" {
		return fmt.Errorf("Brevo alert destination is not configured")
	}
	body := fmt.Sprintf("<h2>%s</h2><p><strong>Severity:</strong> %s</p><p><strong>Event:</strong> %s</p><p>%s</p><p><strong>Reference:</strong> %s</p><p><strong>Created:</strong> %s</p>", html.EscapeString(alert.Title), html.EscapeString(alert.Severity), html.EscapeString(alert.EventType), html.EscapeString(alert.Message), html.EscapeString(alert.Fingerprint), alert.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	return b.mailer.SendOperationalAlert(ctx, b.recipient, "[XAVAULT "+strings.ToUpper(alert.Severity)+"] "+alert.Title, body)
}
