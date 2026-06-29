package users

import "time"

type User struct {
	ID                          string     `json:"id"`
	Email                       string     `json:"email"`
	Password                    string     `json:"password"`
	MailVerified                bool       `json:"mail_verified"`
	EmailVerifiedAt             *time.Time `json:"email_verified_at"`
	VerificationToken           *string    `json:"verification_token"`
	VerificationTokenExpiresAt  *time.Time `json:"verification_token_expires_at"`
	PasswordResetToken          *string    `json:"password_reset_token"`
	PasswordResetTokenExpiresAt *time.Time `json:"password_reset_token_expires_at"`
	RefreshToken                *string    `json:"refresh_token"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}
