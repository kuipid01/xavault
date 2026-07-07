package subscription

import "github.com/google/uuid"

type CreateSubscriptionRequest struct {
	PlanID uuid.UUID `json:"plan_id" validate:"required"`
}

type CreateSubscriptionResponse struct {
	Plan SubscriptionPlan `json:"plan"`
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
