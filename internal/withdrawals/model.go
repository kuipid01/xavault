package withdrawals

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	WithdrawalStatusPending   = "pending"
	WithdrawalStatusPaid      = "paid"
	WithdrawalStatusFailed    = "failed"
	WithdrawalStatusCancelled = "cancelled"

	WithdrawalSourcePlanUnboxed = "plan_unboxed"
	WithdrawalSourceEnvelope    = "envelope"
)

type Withdrawal struct {
	ID string `gorm:"type:uuid;primaryKey" json:"id"`

	UserID   string `gorm:"type:uuid;not null;index" json:"user_id"`
	WalletID string `gorm:"type:uuid;not null;index" json:"wallet_id"`
	PlanID   string `gorm:"type:uuid;not null;index" json:"plan_id"`

	EnvelopeID *string `gorm:"type:uuid;index" json:"envelope_id"`

	SourceType string `gorm:"type:varchar(40);not null;index" json:"source_type"`
	Status     string `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`

	Amount   int64  `gorm:"not null" json:"amount"`
	Currency string `gorm:"type:varchar(10);not null;default:'NGN'" json:"currency"`

	Reason      string `gorm:"type:varchar(255)" json:"reason"`
	Reference   string `gorm:"type:varchar(120);not null;uniqueIndex" json:"reference"`
	Description string `gorm:"type:varchar(255)" json:"description"`

	PaidAt      *time.Time `json:"paid_at"`
	FailedAt    *time.Time `json:"failed_at"`
	CancelledAt *time.Time `json:"cancelled_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (w *Withdrawal) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}

	if w.Status == "" {
		w.Status = WithdrawalStatusPending
	}

	if w.Currency == "" {
		w.Currency = "NGN"
	}

	return nil
}
