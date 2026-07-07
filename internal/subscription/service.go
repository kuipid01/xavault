package subscription

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type SubscriptionService struct {
	subRepo *SubscriptionRepository
}

func NewSubscriptionService(subRepo *SubscriptionRepository) *SubscriptionService {
	return &SubscriptionService{subRepo: subRepo}
}

func (s *SubscriptionService) CreateSubscription(userID uuid.UUID, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	// Let's assume we fetch the plan from repo or hardcode for now if there's no GetPlan method.
	// For now, return a placeholder plan or fetch from DB.
	var plan SubscriptionPlan
	err := s.subRepo.db.First(&plan, "id = ?", req.PlanID).Error
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}
	return &CreateSubscriptionResponse{Plan: plan}, nil
}

func (s *SubscriptionService) ActivateSubscription(userIDStr, planIDStr, providerRef string) error {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		return fmt.Errorf("invalid plan id: %w", err)
	}

	var plan SubscriptionPlan
	if err := s.subRepo.db.First(&plan, "id = ?", planID).Error; err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	sub := UserSubscription{
		UserID:             userID,
		PlanID:             planID,
		Status:             SubActive, // Assuming SubActive is a constant
		CurrentPeriodStart: time.Now(),
		CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0), // 1 month
		Provider:           "paystack",
		ProviderRef:        providerRef,
	}

	if err := s.subRepo.db.Create(&sub).Error; err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) GetSubscriptionByID(id uuid.UUID) (*GetSubscriptionResponse, error) {
	return nil, nil
}

func (s *SubscriptionService) GetSubscriptions(query *ListSubscriptionsQuery) (*GetSubscriptionsResponse, error) {
	return nil, nil
}

func (s *SubscriptionService) GetActiveSubscription(userID uuid.UUID) (*GetSubscriptionResponse, error) {
	sub, err := s.subRepo.GetActiveSubscription(userID)
	if err != nil {
		return nil, err
	}
	return &GetSubscriptionResponse{Subscription: *sub}, nil
}

func (s *SubscriptionService) GetSubscriptionHistory() (*GetSubscriptionsResponse, error) {
	return nil, nil
}