package subscription

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	var sub UserSubscription
	tx := s.db
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

func (s *SubscriptionRepository) CanCreateContainer(
	userID uuid.UUID,
	containerType ContainerType,
) error {
	sub, err := s.GetActiveSubscription(userID)
	if err != nil {
		return err
	}

	periodKey := time.Now().Format("2006-01")

	var usage MonthlyUsage
	tx := s.db
	err = tx.Where("user_id = ? AND period_key = ?", userID, periodKey).
		FirstOrCreate(&usage, MonthlyUsage{
			UserID:    userID,
			PeriodKey: periodKey,
		}).Error

	if err != nil {
		return err
	}

	plan := sub.Plan

	switch containerType {
	case ContainerLocked:
		if !unlimitedInt(plan.MaxLockedPlans) && usage.LockedPlansCreated >= plan.MaxLockedPlans {
			return ErrPlanLimitReached
		}

	case ContainerFlexible:
		if !unlimitedInt(plan.MaxFlexibleEnvelopes) && usage.FlexibleEnvelopesCreated >= plan.MaxFlexibleEnvelopes {
			return ErrPlanLimitReached
		}

	case ContainerEmergency:
		if !unlimitedInt(plan.MaxEmergencyVaults) && usage.EmergencyVaultsCreated >= plan.MaxEmergencyVaults {
			return ErrPlanLimitReached
		}
	}

	return nil
}

func (s *SubscriptionRepository) CanAddManagedVolume(
	userID uuid.UUID,
	amountKobo int64,
) error {
	sub, err := s.GetActiveSubscription(userID)
	if err != nil {
		return err
	}

	periodKey := time.Now().Format("2006-01")

	var usage MonthlyUsage
	tx := s.db
	err = tx.Where("user_id = ? AND period_key = ?", userID, periodKey).
		FirstOrCreate(&usage, MonthlyUsage{
			UserID:    userID,
			PeriodKey: periodKey,
		}).Error

	if err != nil {
		return err
	}

	limit := sub.Plan.MaxMonthlyVolumeKobo
	if !unlimitedMoney(limit) && usage.ManagedVolumeKobo+amountKobo > limit {
		return ErrPlanLimitReached
	}

	return nil
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
