package waitlist

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Entry struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	Email     string    `gorm:"type:varchar(320);not null;uniqueIndex" json:"email"`
	Source    string    `gorm:"type:varchar(80);not null;default:'landing_page'" json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (e *Entry) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Source == "" {
		e.Source = "landing_page"
	}
	return nil
}
