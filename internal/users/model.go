package users

import "time"

type User struct {
	ID                          string     `json:"id"`
	Email                       string     `json:"email"`
	Password                    string     `json:"password"`
	MailVerified                bool       `json:"mail_verified"`
	Country                     string     `json:"country"`
	PhoneNumber                 string     `json:"phone_number"`
	EmailVerifiedAt             *time.Time `json:"email_verified_at"`
	VerificationToken           *string    `json:"verification_token"`
	VerificationTokenExpiresAt  *time.Time `json:"verification_token_expires_at"`
	PasswordResetToken          *string    `json:"password_reset_token"`
	PasswordResetTokenExpiresAt *time.Time `json:"password_reset_token_expires_at"`
	RefreshToken                *string    `json:"refresh_token"`
	WithdrawalPinHash           *string    `json:"-"`
	WithdrawalPinResetOTP       *string    `json:"-"`
	WithdrawalPinResetExpiresAt *time.Time `json:"-"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}
