package spendingplans

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PlanStatusDraft     = "draft"
	PlanStatusActive    = "active"
	PlanStatusCompleted = "completed"
	PlanStatusCancelled = "cancelled"
	PlanStatusExpired   = "expired"

	PlanUnboxedPolicyFlexible            = "flexible"
	PlanUnboxedPolicyLockedUntilPlanEnds = "locked_until_plan_ends"
)

type SpendingPlan struct {
	ID       string `gorm:"type:uuid;primaryKey" json:"id"`
	UserID   string `gorm:"type:uuid;not null;index" json:"user_id"`
	WalletID string `gorm:"type:uuid;not null;index" json:"wallet_id"`

	Name     string `gorm:"type:varchar(120);not null" json:"name"`
	Amount   int64  `gorm:"not null" json:"amount"` // store in kobo
	Currency string `gorm:"type:varchar(10);not null;default:'NGN'" json:"currency"`

	Status        string `gorm:"type:varchar(30);not null;default:'draft';index" json:"status"`
	FundingStatus string `gorm:"type:varchar(30);not null;default:'unfunded';index" json:"funding_status"`

	UnboxedBalancePolicy string `gorm:"type:varchar(50);not null;default:'flexible'" json:"unboxed_balance_policy"`

	StartDate time.Time `gorm:"not null" json:"start_date"`
	EndDate   time.Time `gorm:"not null" json:"end_date"`

	ConfirmedAt *time.Time `json:"confirmed_at"`
	FundedAt    *time.Time `json:"funded_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ExpiredAt   *time.Time `json:"expired_at"`

	CancelReason string `gorm:"type:varchar(255)" json:"cancel_reason"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (p *SpendingPlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	if p.Status == "" {
		p.Status = PlanStatusDraft
	}
	if p.FundingStatus == "" {
		p.FundingStatus = PlanFundingStatusUnfunded
	}
	if p.Currency == "" {
		p.Currency = "NGN"
	}

	if p.UnboxedBalancePolicy == "" {
		p.UnboxedBalancePolicy = PlanUnboxedPolicyFlexible
	}

	return nil
}
