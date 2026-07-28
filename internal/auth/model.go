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

type BiometricDevice struct {
	ID           string     `json:"id" gorm:"type:uuid;primaryKey"`
	UserID       string     `json:"user_id" gorm:"type:uuid;not null;index"`
	DeviceID     string     `json:"device_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	DeviceName   string     `json:"device_name" gorm:"type:varchar(128);not null"`
	Platform     string     `json:"platform" gorm:"type:varchar(32);not null"`
	PublicKey    string     `json:"-" gorm:"type:text;not null"`
	KeyAlgorithm string     `json:"key_algorithm" gorm:"type:varchar(16);not null"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	RevokedAt    *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type BiometricChallenge struct {
	ID            string     `json:"id" gorm:"type:uuid;primaryKey"`
	CredentialID  string     `json:"-" gorm:"type:uuid;not null;index"`
	ChallengeHash string     `json:"-" gorm:"type:char(64);not null"`
	ExpiresAt     time.Time  `json:"expires_at" gorm:"not null;index"`
	UsedAt        *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
}
