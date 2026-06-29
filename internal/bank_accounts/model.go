package bankaccounts

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AccountOwnerTypeSelf        = "self"
	AccountOwnerTypeBeneficiary = "beneficiary"

	AccountProviderPaystack = "Paystack"
)

type BankAccount struct {
	ID     string `gorm:"type:uuid;primaryKey" json:"id"`
	UserID string `gorm:"type:uuid;not null;index" json:"user_id"`

	OwnerType string `gorm:"type:varchar(30);not null;index" json:"owner_type"`
	Nickname  string `gorm:"type:varchar(120)" json:"nickname"`

	BankName      string `gorm:"type:varchar(160);not null" json:"bank_name"`
	BankCode      string `gorm:"type:varchar(30);not null;index" json:"bank_code"`
	AccountNumber string `gorm:"type:varchar(20);not null;index" json:"account_number"`
	AccountName   string `gorm:"type:varchar(180);not null" json:"account_name"`

	Provider      string `gorm:"type:varchar(40);not null;default:'Paystack'" json:"provider"`
	RecipientCode string `gorm:"type:varchar(120);not null;index" json:"recipient_code"`
	IsDefault     bool   `gorm:"not null;default:false" json:"is_default"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (a *BankAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}

	if a.Provider == "" {
		a.Provider = AccountProviderPaystack
	}

	if a.OwnerType == "" {
		a.OwnerType = AccountOwnerTypeSelf
	}

	return nil
}
