package reconciliation

import "time"

const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
)

type Control struct {
	ID               uint   `gorm:"primaryKey"`
	ProcessingHalted bool   `gorm:"not null;default:false;index"`
	Reason           string `gorm:"type:text"`
	HaltedAt         *time.Time
	UpdatedAt        time.Time
}

type Finding struct {
	ID         uint       `gorm:"primaryKey"`
	RunID      string     `gorm:"type:uuid;not null;index"`
	Severity   string     `gorm:"type:varchar(20);not null;index"`
	Code       string     `gorm:"type:varchar(80);not null;index"`
	EntityType string     `gorm:"type:varchar(40);not null"`
	EntityID   string     `gorm:"type:varchar(120);index"`
	Expected   string     `gorm:"type:text"`
	Actual     string     `gorm:"type:text"`
	ResolvedAt *time.Time `gorm:"index"`
	CreatedAt  time.Time
}

type OperatorAction struct {
	ID         uint   `gorm:"primaryKey"`
	OperatorID string `gorm:"type:varchar(120);not null;index"`
	Action     string `gorm:"type:varchar(80);not null"`
	Reference  string `gorm:"type:varchar(140);index"`
	Details    string `gorm:"type:text"`
	CreatedAt  time.Time
}

func (Control) TableName() string        { return "reconciliation_controls" }
func (Finding) TableName() string        { return "reconciliation_findings" }
func (OperatorAction) TableName() string { return "reconciliation_operator_actions" }
