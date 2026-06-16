package wallets

import "time"

type Wallet struct {
	ID               string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           string    `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	AvailableBalance int64     `gorm:"not null;default:0" json:"available_balance"`
	LockedBalance    int64     `gorm:"not null;default:0" json:"locked_balance"`
	LedgerBalance    int64     `gorm:"not null;default:0" json:"ledger_balance"`
	Currency         string    `gorm:"not null;default:'NGN'" json:"currency"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type WalletDeposit struct {
	ID       string `gorm:"type:uuid;primaryKey" json:"id"`
	UserID   string `gorm:"type:uuid;not null;index" json:"user_id"`
	WalletID string `gorm:"type:uuid;not null;index" json:"wallet_id"`

	//  FIX: Change type:uuid to type:varchar(100) to allow custom "DEP-..." prefixes
	Reference string `gorm:"type:varchar(100);not null;uniqueIndex" json:"reference"`

	Amount    int64     `gorm:"not null;default:0" json:"amount"`
	Currency  string    `gorm:"not null;default:'NGN'" json:"currency"`
	Status    string    `gorm:"not null;default:'pending'" json:"status"`
	Provider  string    `gorm:"not null;default:'paystack'" json:"provider"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
