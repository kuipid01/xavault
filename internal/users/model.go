package users

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/subscription"
)

type User struct {
	ID                          string     `json:"id"`
	Email                       string     `json:"email"`
	Password                    string     `json:"password"`
	FirstName                   string     `json:"first_name"`
	LastName                    string     `json:"last_name"`
	MailVerified                bool       `json:"mail_verified"`
	Country                     string     `json:"country"`
	PhoneNumber                 string     `json:"phone_number"`
	Nickname                    *string    `json:"nickname" gorm:"uniqueIndex"`
	DateOfBirth                 *time.Time `json:"date_of_birth"`
	Address                     *string    `json:"address"`
	ProfileImageURL             *string    `json:"profile_image_url"`
	ProfileImagePublicID        *string    `json:"-"`
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
	Subscriptions               []subscription.UserSubscription
}

type SecurityQuestion struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID     string    `json:"user_id" gorm:"type:uuid;not null;index"`
	Question   string    `json:"question" gorm:"type:varchar(255);not null"`
	AnswerHash string    `json:"-" gorm:"type:varchar(255);not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
