package users

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/subscription"
)

type User struct {
	ID                          string     `json:"id"`
	Email                       string     `json:"email"`
	Password                    string     `json:"-"`
	FirstName                   string     `json:"first_name"`
	LastName                    string     `json:"last_name"`
	MailVerified                bool       `json:"mail_verified"`
	Country                     string     `json:"country"`
	PhoneNumber                 string     `json:"phone_number"`
	Gender                      *string    `json:"gender"`
	Nickname                    *string    `json:"nickname" gorm:"uniqueIndex"`
	DateOfBirth                 *time.Time `json:"date_of_birth"`
	Address                     *string    `json:"address"`
	ProfileImageURL             *string    `json:"profile_image_url"`
	ProfileImagePublicID        *string    `json:"-"`
	EmailVerifiedAt             *time.Time `json:"email_verified_at"`
	VerificationToken           *string    `json:"-"`
	VerificationTokenExpiresAt  *time.Time `json:"-"`
	PasswordResetToken          *string    `json:"-"`
	PasswordResetTokenExpiresAt *time.Time `json:"-"`
	RefreshToken                *string    `json:"-"`
	WithdrawalPinHash           *string    `json:"-"`
	WithdrawalPinFailedAttempts int        `json:"-" gorm:"not null;default:0"`
	WithdrawalPinLockedAt       *time.Time `json:"-"`
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
