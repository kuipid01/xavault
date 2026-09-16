package withdrawals

import (
	"encoding/json"
	"errors"
	"time"

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

func (r *WithdrawalRepository) FindByUserID(userID string, limit int, offset int) ([]Withdrawal, int64, error) {
	var withdrawals []Withdrawal
	var total int64

	if err := r.db.Model(&Withdrawal{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&withdrawals).Error

	return withdrawals, total, err
}
func (r *WithdrawalRepository) dbOrTx(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}

	return r.db
}
func (r *WithdrawalRepository) SumPaidPlanUnboxedWithdrawalsTx(
	tx *gorm.DB,
	planID string,
	userID string,
) (int64, error) {
	var total int64
	db := r.dbOrTx(tx)
	err := db.
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
	db := r.dbOrTx(tx)

	err := db.
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

func (r *WithdrawalRepository) FindByReferenceForUpdate(
	tx *gorm.DB,
	reference string,
) (*Withdrawal, error) {
	var withdrawal Withdrawal

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reference = ? OR provider_reference = ?", reference, reference).
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

func (r *WithdrawalRepository) HasPendingWithdrawalForSourceTx(
	tx *gorm.DB,
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
) (bool, error) {
	if tx == nil {
		return false, errors.New("transaction is required")
	}

	var count int64

	query := tx.Model(&Withdrawal{}).
		Where("user_id = ? AND plan_id = ? AND source_type = ? AND status IN ?",
			userID,
			planID,
			sourceType,
			[]string{
				WithdrawalStatusPending,
				WithdrawalStatusProcessing,
			},
		)

	if envelopeID == nil || *envelopeID == "" {
		query = query.Where("envelope_id IS NULL")
	} else {
		query = query.Where("envelope_id = ?", *envelopeID)
	}

	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *WithdrawalRepository) FindOpenWithdrawalForSourceTx(
	tx *gorm.DB,
	userID string,
	planID string,
	sourceType string,
	envelopeID *string,
) (*Withdrawal, error) {
	if tx == nil {
		return nil, errors.New("transaction is required")
	}

	var withdrawal Withdrawal

	query := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND plan_id = ? AND source_type = ? AND status IN ?",
			userID,
			planID,
			sourceType,
			[]string{
				WithdrawalStatusPending,
				WithdrawalStatusProcessing,
			},
		)

	if envelopeID == nil || *envelopeID == "" {
		query = query.Where("envelope_id IS NULL")
	} else {
		query = query.Where("envelope_id = ?", *envelopeID)
	}

	err := query.Order("created_at DESC").First(&withdrawal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &withdrawal, nil
}

func (r *WithdrawalRepository) MarkTransferInitiatedWithTx(
	tx *gorm.DB,
	withdrawalID string,
	providerReference string,
	providerTransferCode string,
	providerStatus string,
	rawProviderResponse interface{},
) error {
	if tx == nil {
		return errors.New("transaction is required")
	}

	now := time.Now()

	rawJSON := ""
	if rawProviderResponse != nil {
		b, err := json.Marshal(rawProviderResponse)
		if err == nil {
			rawJSON = string(b)
		}
	}

	return tx.Model(&Withdrawal{}).
		Where("id = ? AND status = ?", withdrawalID, WithdrawalStatusPending).
		Updates(map[string]interface{}{
			"status":                 WithdrawalStatusProcessing,
			"provider_status":        providerStatus,
			"provider_reference":     providerReference,
			"provider_transfer_code": providerTransferCode,
			"provider_response_json": rawJSON,
			"transfer_initiated_at":  &now,
		}).Error
}

func (r *WithdrawalRepository) MarkFailedWithTx(
	tx *gorm.DB,
	withdrawalID string,
	reason string,
) error {
	if tx == nil {
		return errors.New("transaction is required")
	}

	return tx.Model(&Withdrawal{}).
		Where("id = ? AND status IN ?", withdrawalID, []string{WithdrawalStatusPending, WithdrawalStatusProcessing}).
		Updates(map[string]interface{}{
			"status":          WithdrawalStatusFailed,
			"provider_status": "failed",
			"failure_reason":  reason,
			"failed_at":       time.Now(),
		}).Error
}

func (r *WithdrawalRepository) MarkPaidWithTx(
	tx *gorm.DB,
	withdrawalID string,
	providerStatus string,
) error {
	if tx == nil {
		return errors.New("transaction is required")
	}

	now := time.Now()

	return tx.Model(&Withdrawal{}).
		Where("id = ?", withdrawalID).
		Updates(map[string]interface{}{
			"status":          WithdrawalStatusPaid,
			"provider_status": providerStatus,
			"paid_at":         &now,
		}).Error
}

func (r *WithdrawalRepository) MarkReversedWithTx(
	tx *gorm.DB,
	withdrawalID string,
	providerStatus string,
	reason string,
) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	now := time.Now()
	return tx.Model(&Withdrawal{}).Where("id = ?", withdrawalID).Updates(map[string]interface{}{
		"status":          WithdrawalStatusReversed,
		"provider_status": providerStatus,
		"failure_reason":  reason,
		"reversed_at":     &now,
	}).Error
}

// MarkCancelledWithTx marks a user-cancelled withdrawal. Only a pending
// withdrawal whose transfer never reached the provider is eligible, so no
// money has moved and nothing needs refunding.
func (r *WithdrawalRepository) MarkCancelledWithTx(
	tx *gorm.DB,
	withdrawalID string,
	reason string,
) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	now := time.Now()
	return tx.Model(&Withdrawal{}).
		Where("id = ? AND status = ?", withdrawalID, WithdrawalStatusPending).
		Updates(map[string]interface{}{
			"status":          WithdrawalStatusCancelled,
			"provider_status": "cancelled",
			"failure_reason":  reason,
			"cancelled_at":    &now,
		}).Error
}
