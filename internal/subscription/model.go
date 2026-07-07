package subscription

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
type UserSubscription struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;index;not null"`

	PlanID uuid.UUID `gorm:"type:uuid;index;not null"`
	Plan   SubscriptionPlan

	Status SubscriptionStatus `gorm:"index;not null"`

	CurrentPeriodStart time.Time `gorm:"not null"`
	CurrentPeriodEnd   time.Time `gorm:"index;not null"`

	Provider          string
	ProviderRef       string `gorm:"index"`
	CancelAtPeriodEnd bool
}

type MonthlyUsage struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;index;not null"`

	PeriodKey string `gorm:"index;not null"` // example: 2026-02

	ManagedVolumeKobo int64 `gorm:"not null;default:0"`

	LockedPlansCreated       int `gorm:"not null;default:0"`
	FlexibleEnvelopesCreated int `gorm:"not null;default:0"`
	EmergencyVaultsCreated   int `gorm:"not null;default:0"`

	FreeFlexibleWithdrawalsUsed  int `gorm:"not null;default:0"`
	FreeEmergencyWithdrawalsUsed int `gorm:"not null;default:0"`
}

type SubscriptionPlan struct {
	BaseModel

	Code      PlanCode `gorm:"uniqueIndex;not null"`
	Name      string   `gorm:"not null"`
	PriceKobo int64    `gorm:"not null"`
	Currency  string   `gorm:"default:NGN"`

	MaxMonthlyVolumeKobo int64 `gorm:"not null"` // use -1 for unlimited
	MaxLockedPlans       int   `gorm:"not null"` // use -1 for unlimited
	MaxFlexibleEnvelopes int   `gorm:"not null"` // use -1 for unlimited
	MaxEmergencyVaults   int   `gorm:"not null"`

	FreeFlexibleWithdrawalsMonthly  int `gorm:"not null"`
	FreeEmergencyWithdrawalsMonthly int `gorm:"not null"`

	HasSalarySplit     bool
	HasFullAnalytics   bool
	HasSharedEnvelopes bool
	HasAdvancedRules   bool
}
