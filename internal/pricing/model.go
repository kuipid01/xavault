package pricing

import "time"

const (
	OperationDeposit    = "deposit"
	OperationWithdrawal = "withdrawal"
	StatusActive        = "active"
	StatusConsumed      = "consumed"
)

type FeeRule struct {
	ID                          string `gorm:"type:uuid;primaryKey"`
	Provider                    string
	Operation                   string
	Channel                     string
	Currency                    string
	PercentageBasisPoints       int64
	FixedFeeKobo                int64
	FeeWaiverThresholdKobo      *int64
	MaximumFeeKobo              *int64
	MinimumAmountKobo           int64
	MaximumAmountKobo           *int64
	GovernmentLevyKobo          int64
	GovernmentLevyThresholdKobo *int64
	Version                     string
	EffectiveFrom               time.Time
	EffectiveUntil              *time.Time
	Enabled                     bool
}

func (FeeRule) TableName() string { return "provider_fee_rules" }

type Quote struct {
	ID               string     `gorm:"type:uuid;primaryKey" json:"quote_id"`
	UserID           string     `gorm:"type:uuid;index" json:"-"`
	Operation        string     `json:"operation"`
	Provider         string     `json:"provider"`
	Channel          string     `json:"channel"`
	Currency         string     `json:"currency"`
	PrincipalAmount  int64      `json:"principal_amount"`
	ProviderFee      int64      `json:"provider_fee"`
	GovernmentLevy   int64      `json:"government_levy"`
	XavaultSubsidy   int64      `json:"xavault_subsidy"`
	UserFee          int64      `json:"user_fee"`
	GrossCharge      int64      `json:"gross_charge"`
	NetCredit        int64      `json:"net_credit"`
	TotalDeduction   int64      `json:"total_deduction"`
	RecipientAmount  int64      `json:"recipient_amount"`
	AllowanceApplied bool       `json:"allowance_applied"`
	PricingVersion   string     `json:"pricing_version"`
	Status           string     `json:"-"`
	ContextJSON      string     `gorm:"type:jsonb" json:"-"`
	ExpiresAt        time.Time  `json:"expires_at"`
	ConsumedAt       *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"-"`
}

func (Quote) TableName() string { return "fee_quotes" }
