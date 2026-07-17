package envelope

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EnvelopeRepo struct {
	db *gorm.DB
}

func NewEnvelopeRepo(db *gorm.DB) *EnvelopeRepo {
	return &EnvelopeRepo{db: db}
}

func (repo *EnvelopeRepo) WithTransaction(fn func(tx *gorm.DB) error) error {
	return repo.db.Transaction(fn)
}

func (repo *EnvelopeRepo) CreateWithTx(tx *gorm.DB, envelope *Envelope) error {
	return tx.Create(envelope).Error
}

func (repo *EnvelopeRepo) FindByIDPlanIDAndUserID(
	envelopeID string,
	planID string,
	userID string,
) (*Envelope, error) {
	var envelope Envelope

	err := repo.db.
		Where("id = ? AND plan_id = ? AND user_id = ?", envelopeID, planID, userID).
		First(&envelope).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &envelope, nil
}

func (repo *EnvelopeRepo) FindByIDPlanIDAndUserIDForUpdate(
	tx *gorm.DB,
	envelopeID string,
	planID string,
	userID string,
) (*Envelope, error) {
	var envelope Envelope

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND plan_id = ? AND user_id = ?", envelopeID, planID, userID).
		First(&envelope).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &envelope, nil
}

func (repo *EnvelopeRepo) FindByPlanIDAndUserID(planID string, userID string) ([]Envelope, error) {
	var envelopes []Envelope

	err := repo.db.
		Where("plan_id = ? AND user_id = ?", planID, userID).
		Order("created_at ASC").
		Find(&envelopes).Error

	if err != nil {
		return nil, err
	}

	return envelopes, nil
}

func (repo *EnvelopeRepo) dbOrTx(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}

	return repo.db
}

func (repo *EnvelopeRepo) SumTopLevelEnvelopeAmountTx(tx *gorm.DB, planID string) (int64, error) {
	var total int64

	err := repo.dbOrTx(tx).
		Model(&Envelope{}).
		Where("plan_id = ? AND parent_envelope_id IS NULL", planID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	return total, err
}

func (repo *EnvelopeRepo) SumTopLevelEnvelopeAmount(planID string) (int64, error) {
	return repo.SumTopLevelEnvelopeAmountTx(nil, planID)
}

func (repo *EnvelopeRepo) SumTopLevelEnvelopeAmountExcludingTx(
	tx *gorm.DB,
	planID string,
	excludedEnvelopeID string,
) (int64, error) {
	var total int64

	err := tx.
		Model(&Envelope{}).
		Where("plan_id = ? AND parent_envelope_id IS NULL AND id <> ?", planID, excludedEnvelopeID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	return total, err
}

func (repo *EnvelopeRepo) SumChildEnvelopeAmount(parentEnvelopeID string) (int64, error) {
	var total int64

	err := repo.db.
		Model(&Envelope{}).
		Where("parent_envelope_id = ?", parentEnvelopeID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	return total, err
}

func (repo *EnvelopeRepo) SumChildEnvelopeAmountTx(tx *gorm.DB, parentEnvelopeID string) (int64, error) {
	var total int64

	err := tx.
		Model(&Envelope{}).
		Where("parent_envelope_id = ?", parentEnvelopeID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	return total, err
}

func (repo *EnvelopeRepo) SumChildEnvelopeAmountExcludingTx(
	tx *gorm.DB,
	parentEnvelopeID string,
	excludedEnvelopeID string,
) (int64, error) {
	var total int64

	err := tx.
		Model(&Envelope{}).
		Where("parent_envelope_id = ? AND id <> ?", parentEnvelopeID, excludedEnvelopeID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	return total, err
}

func (repo *EnvelopeRepo) CountChildrenTx(tx *gorm.DB, envelopeID string) (int64, error) {
	var count int64

	err := tx.
		Model(&Envelope{}).
		Where("parent_envelope_id = ?", envelopeID).
		Count(&count).Error

	return count, err
}

func (repo *EnvelopeRepo) CountActiveByPlanIDTx(tx *gorm.DB, planID string) (int64, error) {
	var count int64

	err := tx.
		Model(&Envelope{}).
		Where("plan_id = ? AND status = ?", planID, EnvelopeStatusActive).
		Count(&count).Error

	return count, err
}

func (repo *EnvelopeRepo) UpdateWithTx(tx *gorm.DB, envelope *Envelope) error {
	return tx.Save(envelope).Error
}

func (repo *EnvelopeRepo) DeleteWithTx(tx *gorm.DB, envelope *Envelope) error {
	return tx.Delete(envelope).Error
}
