package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type BrevoMailer struct {
	APIKey      string
	SenderName  string
	SenderEmail string
	Client      *http.Client
	Endpoint    string
}

func NewBrevoMailer() *BrevoMailer {

	return &BrevoMailer{
		APIKey:      os.Getenv("BREVO_API_KEY"),
		SenderName:  os.Getenv("BREVO_SENDER_NAME"),
		SenderEmail: os.Getenv("BREVO_SENDER_EMAIL"),
		Client:      &http.Client{Timeout: 10 * time.Second},
		Endpoint:    "https://api.brevo.com/v3/smtp/email",
	}
}

type BrevoErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BrevoRecipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type BrevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoSendEmailPayload struct {
	Sender      BrevoSender      `json:"sender"`
	To          []BrevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HTMLContent string           `json:"htmlContent"`
	Tags        []string         `json:"tags,omitempty"`
}

func (b *BrevoMailer) sendHTMLMail(
	ctx context.Context,
	toEmail string,
	toName string,
	subject string,
	htmlContent string,
	tags []string,
) error {
	payload := BrevoSendEmailPayload{
		Sender: BrevoSender{
			Name:  b.SenderName,
			Email: b.SenderEmail,
		},
		To: []BrevoRecipient{
			{
				Email: toEmail,
				Name:  toName,
			},
		},
		Subject:     subject,
		HTMLContent: htmlContent,
		Tags:        tags,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		b.Endpoint,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", b.APIKey)

	res, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		respBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return fmt.Errorf("brevo email failed with status %d (could not read response body: %v)", res.StatusCode, readErr)
		}

		var brevoErr BrevoErrorResponse
		if err := json.Unmarshal(respBytes, &brevoErr); err == nil && brevoErr.Message != "" {
			return fmt.Errorf("brevo API error [%d %s]: %s", res.StatusCode, brevoErr.Code, brevoErr.Message)
		}

		return fmt.Errorf("brevo API error [%d]: %s", res.StatusCode, string(respBytes))
	}

	return nil
}

func (b *BrevoMailer) SendOperationalAlert(ctx context.Context, email, subject, htmlContent string) error {
	return b.sendHTMLMail(ctx, email, "XAVAULT Operations", subject, htmlContent, []string{"operational-alert"})
}

func (b *BrevoMailer) SendMailVerification(email, otp string) error {
	data := struct {
		Name          string
		AppName       string
		OTP           string
		ExpiryMinutes int
	}{
		Name:          "",
		AppName:       "XAVAULT",
		OTP:           otp,
		ExpiryMinutes: 30,
	}

	htmlContent, err := RenderEmailTemplate("verification.html", data)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = b.sendHTMLMail(context.Background(),
		email,
		"",
		"Please Verify Your Email",
		htmlContent,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	return err
}
func (b *BrevoMailer) SendForgetPasswordMail(email, otp string) error {
	data := struct {
		Name          string
		AppName       string
		OTP           string
		ExpiryMinutes int
	}{
		Name:          "",
		AppName:       "Salary Vault",
		OTP:           otp,
		ExpiryMinutes: 30,
	}

	htmlContent, err := RenderEmailTemplate("forget_password.html", data)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = b.sendHTMLMail(context.Background(),
		email,
		"",
		"Reset Your Password",
		htmlContent,
		nil,
	)
	return err
}

func (b *BrevoMailer) SendWithdrawalPinOTP(email string, otp string) error {
	data := struct {
		Name          string
		AppName       string
		OTP           string
		ExpiryMinutes int
	}{
		Name:          "",
		AppName:       "Salary Vault",
		OTP:           otp,
		ExpiryMinutes: 10,
	}

	htmlContent, err := RenderEmailTemplate("withdrawal_pin_otp.html", data)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return b.sendHTMLMail(context.Background(),
		email,
		"",
		"Reset Your Withdrawal PIN",
		htmlContent,
		nil,
	)
}
