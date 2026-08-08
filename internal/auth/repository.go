package auth

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) UpsertTemporaryUser(record *TemporaryUser) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"token_hash",
			"expires_at",
			"verified",
			"verified_at",
			"updated_at",
		}),
	}).Create(record).Error
}

func (r *AuthRepository) FindTemporaryUserByEmail(email string) (*TemporaryUser, error) {
	var record TemporaryUser
	result := r.db.Where("email = ?", email).Limit(1).Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &record, nil
}

func (r *AuthRepository) MarkTemporaryUserVerified(email string, verifiedAt time.Time) error {
	return r.db.Model(&TemporaryUser{}).Where("email = ?", email).Updates(map[string]interface{}{
		"verified":    true,
		"verified_at": verifiedAt,
		"token_hash":  "",
	}).Error
}

func (r *AuthRepository) DeleteTemporaryUser(email string) error {
	return r.db.Where("email = ?", email).Delete(&TemporaryUser{}).Error
}
