package subscription

import (
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
