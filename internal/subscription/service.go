package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
)

type SubscriptionService struct {
	subRepo         *SubscriptionRepository
	paymentProvider paymentproviders.PaymentProvider
}

func planTier(code PlanCode) int {
	switch code {
	case PlanFree:
		return 0
	case PlanLite:
		return 1
	case PlanPremium:
		return 2
	case PlanPower:
		return 3
	default:
		return -1
	}
}

func NewSubscriptionService(
	subRepo *SubscriptionRepository,
	paymentProvider paymentproviders.PaymentProvider,
) *SubscriptionService {
	return &SubscriptionService{
		subRepo:         subRepo,
		paymentProvider: paymentProvider,
	}
}

// CreateSubscription initialises a Paystack payment and creates a pending subscription row.
// The subscription becomes active only after the webhook confirms payment via ActivateSubscription.
func (s *SubscriptionService) CreateSubscription(
	ctx context.Context,
	userID uuid.UUID,
	userEmail string,
	req *CreateSubscriptionRequest,
) (*CreateSubscriptionResponse, error) {
	// 1. Fetch plan from DB
	var plan SubscriptionPlan
	if err := s.subRepo.db.First(&plan, "id = ?", req.PlanID).Error; err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	activeSub, err := s.subRepo.GetActiveSubscription(userID)
	if err == nil {
		if planTier(plan.Code) <= planTier(activeSub.Plan.Code) {
			return nil, ErrPlanNotUpgrade
		}
	} else if !errors.Is(err, ErrSubscriptionRequired) {
		return nil, err
	}

	// 2. Build a unique Paystack reference
	reference := fmt.Sprintf("SUB_%s_%s_%d", userID, plan.ID, time.Now().UnixMilli())

	// 3. Initiate deposit with Paystack, embedding subscription metadata
	depositBody := &paymentproviders.DepositBody{
		Amount:    plan.PriceKobo,
		Email:     userEmail,
		Currency:  plan.Currency,
		Reference: reference,
		Metadata: map[string]interface{}{
			"is_subscription": true,
			"plan_id":         plan.ID.String(),
			"user_id":         userID.String(),
		},
	}

	depositResp, err := s.paymentProvider.InitiateDeposit(ctx, depositBody)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate payment: %w", err)
	}
	fmt.Println("response data:", depositResp)
	// Extract authorization_url from the Paystack response data
	resp := depositResp.Data.(map[string]interface{})
	data := resp["data"].(map[string]interface{})
	paymentURL := data["authorization_url"].(string)

	// 4. Create a pending subscription row – will be activated by the webhook
	pending := &UserSubscription{
		BaseModel: BaseModel{
			ID: uuid.New(),
		},
		UserID:             userID,
		PlanID:             plan.ID,
		Status:             SubPending,
		CurrentPeriodStart: time.Time{}, // set on activation
		CurrentPeriodEnd:   time.Time{}, // set on activation
		Provider:           "paystack",
		ProviderRef:        reference,
		PaymentReference:   reference,
	}

	if err := s.subRepo.CreatePendingSubscription(pending); err != nil {
		return nil, fmt.Errorf("failed to create pending subscription: %w", err)
	}

	return &CreateSubscriptionResponse{
		Plan:             plan,
		PaymentURL:       paymentURL,
		PaymentReference: reference,
	}, nil
}

// ActivateSubscription is called from the Paystack charge.success webhook.
// It finds the existing pending subscription by the Paystack reference and activates it.
func (s *SubscriptionService) ActivateSubscription(userIDStr, planIDStr, providerRef string) error {
	if err := s.subRepo.ActivateByReference(providerRef); err != nil {
		return fmt.Errorf("activate subscription: %w", err)
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

func (s *SubscriptionService) GetPlans() ([]SubscriptionPlan, error) {
	return s.subRepo.GetPlans()
}
