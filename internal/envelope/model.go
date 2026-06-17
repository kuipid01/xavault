package envelope

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	EnvelopeStatusActive    = "active"
	EnvelopeStatusCancelled = "cancelled"
	EnvelopeStatusCompleted = "completed"

	EnvelopeUnboxedPolicyFlexible              = "flexible"
	EnvelopeUnboxedPolicyLockedUntilParentEnds = "locked_until_parent_ends"
)

type Envelope struct {
	ID string `gorm:"type:uuid;primaryKey" json:"id"`

	UserID string `gorm:"type:uuid;not null;index" json:"user_id"`
	PlanID string `gorm:"type:uuid;not null;index" json:"plan_id"`

	ParentEnvelopeID *string `gorm:"type:uuid;index" json:"parent_envelope_id"`

	Name   string `gorm:"type:varchar(120);not null" json:"name"`
	Amount int64  `gorm:"not null" json:"amount"`

	Status string `gorm:"type:varchar(30);not null;default:'active';index" json:"status"`

	UnboxedBalancePolicy string `gorm:"type:varchar(50);not null;default:'flexible'" json:"unboxed_balance_policy"`

	StartDate time.Time `gorm:"not null" json:"start_date"`
	EndDate   time.Time `gorm:"not null" json:"end_date"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (e *Envelope) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}

	if e.Status == "" {
		e.Status = EnvelopeStatusActive
	}

	if e.UnboxedBalancePolicy == "" {
		e.UnboxedBalancePolicy = EnvelopeUnboxedPolicyFlexible
	}

	return nil
}
