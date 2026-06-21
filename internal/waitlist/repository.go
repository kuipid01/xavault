package waitlist

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrFind(entry *Entry) (*Entry, bool, error) {
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(entry)
	if result.Error != nil {
		return nil, false, result.Error
	}

	if result.RowsAffected == 1 {
		return entry, true, nil
	}

	var existing Entry
	if err := r.db.Where("email = ?", entry.Email).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, err
		}
		return nil, false, err
	}

	return &existing, false, nil
}
