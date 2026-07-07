package subscription

import "errors"

var (
	ErrSubscriptionRequired = errors.New("active subscription required")
	ErrPlanLimitReached     = errors.New("plan limit reached, upgrade required")
)
