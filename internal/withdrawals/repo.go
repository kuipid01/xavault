package withdrawals

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WithdrawalRepository struct {
	db *gorm.DB
}

func NewWithdrawalRepository(db *gorm.DB) *WithdrawalRepository {
	return &WithdrawalRepository{db: db}
}

func (r *WithdrawalRepository) WithTransaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *WithdrawalRepository) CreateWithTx(tx *gorm.DB, withdrawal *Withdrawal) error {
	if withdrawal == nil {
		return errors.New("withdrawal cannot be nil")
	}

	return tx.Create(withdrawal).Error
}

func (r *WithdrawalRepository) FindByIDAndUserID(withdrawalID string, userID string) (*Withdrawal, error) {
	var withdrawal Withdrawal

	err := r.db.
		Where("id = ? AND user_id = ?", withdrawalID, userID).
		First(&withdrawal).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &withdrawal, nil
}

func (r *WithdrawalRepository) FindByPlanIDAndUserID(planID string, userID string) ([]Withdrawal, error) {
	var withdrawals []Withdrawal

	err := r.db.
		Where("plan_id = ? AND user_id = ?", planID, userID).
		Order("created_at DESC").
		Find(&withdrawals).Error

	if err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func (r *WithdrawalRepository) SumPaidPlanUnboxedWithdrawalsTx(
	tx *gorm.DB,
	planID string,
	userID string,
) (int64, error) {
	var total int64

	err := tx.
		Model(&Withdrawal{}).
		Where(
			"plan_id = ? AND user_id = ? AND source_type = ? AND status = ?",
			planID,
			userID,
			WithdrawalSourcePlanUnboxed,
			WithdrawalStatusPaid,
		).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, err
	}

	return total, nil
}

func (r *WithdrawalRepository) SumPaidEnvelopeWithdrawalsTx(
	tx *gorm.DB,
	envelopeID string,
	userID string,
) (int64, error) {
	var total int64

	err := tx.
		Model(&Withdrawal{}).
		Where(
			"envelope_id = ? AND user_id = ? AND source_type = ? AND status = ?",
			envelopeID,
			userID,
			WithdrawalSourceEnvelope,
			WithdrawalStatusPaid,
		).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, err
	}

	return total, nil
}

func (r *WithdrawalRepository) FindByIDForUpdate(
	tx *gorm.DB,
	withdrawalID string,
) (*Withdrawal, error) {
	var withdrawal Withdrawal

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", withdrawalID).
		First(&withdrawal).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &withdrawal, nil
}

func (r *WithdrawalRepository) UpdateWithTx(tx *gorm.DB, withdrawal *Withdrawal) error {
	if withdrawal == nil {
		return errors.New("withdrawal cannot be nil")
	}

	return tx.Save(withdrawal).Error
}
