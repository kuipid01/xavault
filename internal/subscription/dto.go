package subscription

import "github.com/google/uuid"

type CreateSubscriptionRequest struct {
	PlanID uuid.UUID `json:"plan_id" validate:"required"`
	Email  string    `json:"email"` // user email for Paystack, can be fetched server-side
}

// CreateSubscriptionResponse is returned after a subscription is initiated.
// The frontend must redirect to PaymentURL to complete payment.
type CreateSubscriptionResponse struct {
	Plan             SubscriptionPlan `json:"plan"`
	PaymentURL       string           `json:"payment_url"`
	PaymentReference string           `json:"payment_reference"`
}

type GetSubscriptionResponse struct {
	Subscription UserSubscription `json:"subscription"`
}

type ListSubscriptionsQuery struct {
	UserID *uuid.UUID
}

type GetSubscriptionsResponse struct {
	Subscriptions []UserSubscription `json:"subscriptions"`
}
