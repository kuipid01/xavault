package subscription

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func assignUUID(id *uuid.UUID) {
	if id != nil && *id == uuid.Nil {
		*id = uuid.New()
	}
}

type UserSubscription struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;index;not null"`

	PlanID uuid.UUID        `gorm:"type:uuid;index;not null"`
	Plan   SubscriptionPlan `gorm:"foreignKey:PlanID"`

	Status SubscriptionStatus `gorm:"index;not null"`

	CurrentPeriodStart time.Time `gorm:"not null"`
	CurrentPeriodEnd   time.Time `gorm:"index;not null"`

	Provider         string `gorm:"type:varchar(50)"`
	ProviderRef      string `gorm:"index"`
	PaymentReference string `gorm:"index"`

	CancelAtPeriodEnd bool `gorm:"not null;default:false"`
}

func (u *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	assignUUID(&u.ID)
	return nil
}

type MonthlyUsage struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_monthly_usage_user_period"`

	PeriodKey string `gorm:"type:varchar(7);not null;uniqueIndex:idx_monthly_usage_user_period"`

	ManagedVolumeKobo int64 `gorm:"not null;default:0"`

	LockedPlansCreated       int `gorm:"not null;default:0"`
	FlexibleEnvelopesCreated int `gorm:"not null;default:0"`
	EmergencyVaultsCreated   int `gorm:"not null;default:0"`

	FreeFlexibleWithdrawalsUsed  int `gorm:"not null;default:0"`
	FreeEmergencyWithdrawalsUsed int `gorm:"not null;default:0"`
}

func (m *MonthlyUsage) BeforeCreate(tx *gorm.DB) error {
	assignUUID(&m.ID)
	return nil
}

type SubscriptionPlan struct {
	BaseModel

	Code      PlanCode `gorm:"uniqueIndex;not null"`
	Name      string   `gorm:"not null"`
	PriceKobo int64    `gorm:"not null"`
	Currency  string   `gorm:"type:varchar(3);not null;default:NGN"`

	MaxMonthlyVolumeKobo int64 `gorm:"not null"`
	MaxLockedPlans       int   `gorm:"not null"`
	MaxFlexibleEnvelopes int   `gorm:"not null"`
	MaxEmergencyVaults   int   `gorm:"not null"`

	FreeFlexibleWithdrawalsMonthly  int `gorm:"not null"`
	FreeEmergencyWithdrawalsMonthly int `gorm:"not null"`

	HasSalarySplit     bool `gorm:"not null;default:false"`
	HasFullAnalytics   bool `gorm:"not null;default:false"`
	HasSharedEnvelopes bool `gorm:"not null;default:false"`
	HasAdvancedRules   bool `gorm:"not null;default:false"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	assignUUID(&p.ID)
	return nil
}
