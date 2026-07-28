package wallets

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/users"
)

type Wallet struct {
	ID                     string `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                 string `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	AvailableBalance       int64  `gorm:"not null;default:0" json:"available_balance"`
	LockedBalance          int64  `gorm:"not null;default:0" json:"locked_balance"`
	LedgerBalance          int64  `gorm:"not null;default:0" json:"ledger_balance"`
	Currency               string `gorm:"not null;default:'NGN'" json:"currency"`
	PaystackCustomerCode   string `gorm:"type:varchar(120)" json:"paystack_customer_code"`
	DedicatedAccountID     string `gorm:"type:varchar(80)" json:"dedicated_account_id"`
	DedicatedAccountNumber string `gorm:"type:varchar(20)" json:"dedicated_account_number"`
	DedicatedAccountName   string `gorm:"type:varchar(180)" json:"dedicated_account_name"`
	DedicatedBankName      string `gorm:"type:varchar(160)" json:"dedicated_bank_name"`
	DedicatedBankSlug      string `gorm:"type:varchar(80)" json:"dedicated_bank_slug"`

	User users.User `gorm:"foreignKey:UserID;references:ID" json:"user"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

type WalletLedger struct {
	ID       string `gorm:"type:uuid;primaryKey" json:"id"`
	UserID   string `gorm:"type:uuid;not null;index" json:"user_id"`
	WalletID string `gorm:"type:uuid;not null;index" json:"wallet_id"`

	Currency string `gorm:"not null;default:'NGN'" json:"currency"`

	Reference string `gorm:"not null;uniqueIndex" json:"reference"`

	Type string `gorm:"not null;index" json:"type"`
	// credit, debit, lock, unlock

	Source string `gorm:"not null;index" json:"source"`
	// deposit, withdrawal, spending_plan

	SourceID string `json:"source_id"`

	Amount        int64 `gorm:"not null" json:"amount"`
	BalanceBefore int64 `gorm:"not null" json:"balance_before"`
	BalanceAfter  int64 `gorm:"not null" json:"balance_after"`

	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
