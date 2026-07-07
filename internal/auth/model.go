package auth

import "time"

type TemporaryUser struct {
	ID         string     `json:"id" gorm:"type:uuid;primaryKey"`
	Email      string     `json:"email" gorm:"uniqueIndex;not null"`
	TokenHash  string     `json:"-" gorm:"not null"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null"`
	Verified   bool       `json:"verified" gorm:"not null;default:false"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
