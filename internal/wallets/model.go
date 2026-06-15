package wallets

import "time"

type Wallet struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	Balance   int64     `gorm:"not null;default:0" json:"balance"`
	Currency  string    `gorm:"not null;default:'NGN'" json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
