package subscription

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"gorm.io/gorm"
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

	// 3. Initiate deposit with Paystack, embedding subscription metadata.
	// The callback URL matches the deposit flow so the in-app checkout can
	// detect the redirect and verify immediately after payment. cancel_action
	// mirrors it with a marker so a customer-initiated cancel is distinguishable
	// from a paid redirect (same scheme as the deposit flow).
	callbackURL := fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_LOCAL_URL"))
	if os.Getenv("APP_ENV") == "production" {
		callbackURL = fmt.Sprintf("%s/dashboard/wallet/deposit/callback", os.Getenv("APP_URL"))
	}
	depositBody := &paymentproviders.DepositBody{
		Amount:       plan.PriceKobo,
		Email:        userEmail,
		Currency:     plan.Currency,
		Reference:    reference,
		CallbackURL:  callbackURL,
		// Top-level cancel_action so the in-app checkout WebView can detect a
		// customer cancel and return to the upgrade prompt instead of dangling.
		CancelAction: callbackURL + "?status=cancelled",
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
	paymentURL, err := subscriptionAuthorizationURL(depositResp.Data)
	if err != nil {
		return nil, err
	}

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

// DismissSubscriptionPayment cancels an abandoned subscription checkout so the
// client stops checking and the stale pending row is cleaned up. Webhook
// activation still works if the user completes payment anyway.
func (s *SubscriptionService) DismissSubscriptionPayment(userID uuid.UUID, reference string) (*UserSubscription, error) {
	return s.subRepo.DismissPendingSubscription(userID, reference)
}

// ActivateSubscription is called from the Paystack charge.success webhook.
// It finds the existing pending subscription by the Paystack reference and activates it.
func (s *SubscriptionService) ActivateSubscription(userIDStr, planIDStr, providerRef string, amount int64, currency string) error {
	return s.activateSubscription(nil, userIDStr, planIDStr, providerRef, amount, currency)
}

func (s *SubscriptionService) ActivateSubscriptionTx(tx *gorm.DB, userIDStr, planIDStr, providerRef string, amount int64, currency string) error {
	return s.activateSubscription(tx, userIDStr, planIDStr, providerRef, amount, currency)
}

func (s *SubscriptionService) activateSubscription(tx *gorm.DB, userIDStr, planIDStr, providerRef string, amount int64, currency string) error {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errors.New("invalid subscription user metadata")
	}
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		return errors.New("invalid subscription plan metadata")
	}
	if amount <= 0 {
		return errors.New("invalid subscription payment amount")
	}
	var activateErr error
	if tx == nil {
		activateErr = s.subRepo.ActivateVerified(providerRef, userID, planID, amount, currency)
	} else {
		activateErr = s.subRepo.ActivateVerifiedTx(tx, providerRef, userID, planID, amount, currency)
	}
	if activateErr != nil {
		return fmt.Errorf("activate subscription: %w", activateErr)
	}
	return nil
}

func subscriptionAuthorizationURL(payload interface{}) (string, error) {
	root, ok := payload.(map[string]interface{})
	if !ok || root == nil {
		return "", errors.New("payment provider returned an invalid response")
	}
	data := root
	if nested, ok := root["data"].(map[string]interface{}); ok {
		data = nested
	}
	url, ok := data["authorization_url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return "", errors.New("payment provider response is missing authorization_url")
	}
	return strings.TrimSpace(url), nil
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
