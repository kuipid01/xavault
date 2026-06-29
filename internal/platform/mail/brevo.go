package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type BrevoMailer struct {
	APIKey      string
	SenderName  string
	SenderEmail string
}

func NewBrevoMailer() *BrevoMailer {

	return &BrevoMailer{
		APIKey:      os.Getenv("BREVO_API_KEY"),
		SenderName:  os.Getenv("BREVO_SENDER_NAME"),
		SenderEmail: os.Getenv("BREVO_SENDER_EMAIL"),
	}
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

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.brevo.com/v3/smtp/email",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", b.APIKey)

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("brevo email failed with status: %d", res.StatusCode)
	}

	return nil
}

func (b *BrevoMailer) SendMailVerification(email, otp string) error {
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

	htmlContent, err := RenderEmailTemplate("verification.html", data)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = b.sendHTMLMail(
		email,
		"",
		"Please Verify Your Email",
		htmlContent,
		nil,
	)

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

	err = b.sendHTMLMail(
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

	return b.sendHTMLMail(
		email,
		"",
		"Reset Your Withdrawal PIN",
		htmlContent,
		nil,
	)
}
