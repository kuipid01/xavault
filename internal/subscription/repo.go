package subscription

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// CreatePendingSubscription inserts a new subscription row in pending state.
// The row will be activated by ActivateByReference once Paystack confirms payment.
func (s *SubscriptionRepository) CreatePendingSubscription(sub *UserSubscription) error {
	return s.db.Create(sub).Error
}

// GetPendingByReference finds a pending subscription by its Paystack transaction reference.
func (s *SubscriptionRepository) GetPendingByReference(reference string) (*UserSubscription, error) {
	var sub UserSubscription
	err := s.db.
		Preload("Plan").
		Where("payment_reference = ? AND status = ?", reference, SubPending).
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no pending subscription found for reference %s", reference)
		}
		return nil, err
	}
	return &sub, nil
}

// ActivateByReference finds the pending subscription by reference and marks it active.
func (s *SubscriptionRepository) ActivateByReference(reference string) error {
	now := time.Now()
	result := s.db.Model(&UserSubscription{}).
		Where("payment_reference = ? AND status = ?", reference, SubPending).
		Updates(map[string]interface{}{
			"status":               SubActive,
			"current_period_start": now,
			"current_period_end":   now.AddDate(0, 1, 0), // 1 month
		})
	if result.Error != nil {
		return fmt.Errorf("failed to activate subscription: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no pending subscription found for reference %s", reference)
	}
	return nil
}

// GetActiveSubscription returns the user's current active subscription.
func (s *SubscriptionRepository) GetActiveSubscription(userID uuid.UUID) (*UserSubscription, error) {
	return s.getActiveSubscription(s.db, userID)
}

func (s *SubscriptionRepository) getActiveSubscription(tx *gorm.DB, userID uuid.UUID) (*UserSubscription, error) {
	var sub UserSubscription
	err := tx.
		Preload("Plan").
		Where("user_id = ? AND status = ? AND current_period_end > ?", userID, SubActive, time.Now()).
		Order("created_at DESC").
		First(&sub).Error

	if err != nil {
		return nil, ErrSubscriptionRequired
	}

	return &sub, nil
}

func unlimitedInt(limit int) bool {
	return limit < 0
}

func unlimitedMoney(limit int64) bool {
	return limit < 0
}

func (s *SubscriptionRepository) CanCreatePlan(
	userID uuid.UUID,
	currentPlanCount int64,
) error {
	sub, err := s.GetActiveSubscription(userID)
	if err != nil {
		return err
	}

	limit := sub.Plan.MaxLockedPlans
	if !unlimitedInt(limit) && currentPlanCount >= int64(limit) {
		return ErrPlanLimitReached
	}

	return nil
}

func (s *SubscriptionRepository) CanCreateEnvelope(
	userID uuid.UUID,
	currentEnvelopeCount int64,
) error {
	sub, err := s.GetActiveSubscription(userID)
	if err != nil {
		return err
	}

	limit := sub.Plan.MaxFlexibleEnvelopes
	if !unlimitedInt(limit) && currentEnvelopeCount >= int64(limit) {
		return ErrPlanLimitReached
	}

	return nil
}

func (s *SubscriptionRepository) CanAddMonthlyVolume(
	userID uuid.UUID,
	currentVolumeKobo int64,
	amountKobo int64,
) error {
	sub, err := s.GetActiveSubscription(userID)
	if err != nil {
		return err
	}

	limit := sub.Plan.MaxMonthlyVolumeKobo
	if !unlimitedMoney(limit) && currentVolumeKobo+amountKobo > limit {
		return ErrPlanLimitReached
	}

	return nil
}

func (s *SubscriptionRepository) CanAddManagedVolume(
	userID uuid.UUID,
	amountKobo int64,
) error {
	return s.canAddManagedVolume(s.db, userID, amountKobo)
}

func (s *SubscriptionRepository) canAddManagedVolume(
	tx *gorm.DB,
	userID uuid.UUID,
	amountKobo int64,
) error {
	sub, err := s.getActiveSubscription(tx, userID)
	if err != nil {
		return err
	}

	usage, err := s.getOrCreateMonthlyUsage(tx, userID)
	if err != nil {
		return err
	}

	limit := sub.Plan.MaxMonthlyVolumeKobo
	if !unlimitedMoney(limit) && usage.ManagedVolumeKobo+amountKobo > limit {
		return ErrPlanLimitReached
	}

	return nil
}

func (s *SubscriptionRepository) ReserveManagedVolume(
	tx *gorm.DB,
	userID uuid.UUID,
	amountKobo int64,
) error {
	if tx == nil {
		return s.db.Transaction(func(tx *gorm.DB) error {
			return s.ReserveManagedVolume(tx, userID, amountKobo)
		})
	}

	if err := s.canAddManagedVolume(tx, userID, amountKobo); err != nil {
		return err
	}

	usage, err := s.getOrCreateMonthlyUsage(tx, userID)
	if err != nil {
		return err
	}

	usage.ManagedVolumeKobo += amountKobo
	return tx.Save(usage).Error
}

func (s *SubscriptionRepository) getOrCreateMonthlyUsage(
	tx *gorm.DB,
	userID uuid.UUID,
) (*MonthlyUsage, error) {
	periodKey := time.Now().Format("2006-01")

	var usage MonthlyUsage
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND period_key = ?", userID, periodKey).
		First(&usage).Error

	if err == nil {
		return &usage, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	usage = MonthlyUsage{
		UserID:    userID,
		PeriodKey: periodKey,
	}

	if err := tx.Create(&usage).Error; err != nil {
		return nil, err
	}

	return &usage, nil
}

func (s *SubscriptionRepository) GetPlans() ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan

	err := s.db.
		Order(`
			CASE code
				WHEN 'free' THEN 1
				WHEN 'lite' THEN 2
				WHEN 'premium' THEN 3
				WHEN 'power' THEN 4
				ELSE 5
			END
		`).
		Find(&plans).Error

	return plans, err
}
