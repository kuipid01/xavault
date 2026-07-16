package subscription

type PlanCode string

const (
	PlanFree    PlanCode = "free"
	PlanLite    PlanCode = "lite"
	PlanPremium PlanCode = "premium"
	PlanPower   PlanCode = "power"
)

type ContainerType string

const (
	ContainerLocked    ContainerType = "locked_plan"
	ContainerFlexible  ContainerType = "flexible_envelope"
	ContainerEmergency ContainerType = "emergency_vault"
)

type ContainerStatus string

const (
	ContainerActive    ContainerStatus = "active"
	ContainerMatured   ContainerStatus = "matured"
	ContainerClosed    ContainerStatus = "closed"
	ContainerSuspended ContainerStatus = "suspended"
)

type TxStatus string

const (
	StatusPending  TxStatus = "pending"
	StatusSuccess  TxStatus = "success"
	StatusFailed   TxStatus = "failed"
	StatusReversed TxStatus = "reversed"
)

type SubscriptionStatus string

const (
	SubPending  SubscriptionStatus = "pending"
	SubActive   SubscriptionStatus = "active"
	SubPastDue  SubscriptionStatus = "past_due"
	SubCanceled SubscriptionStatus = "canceled"
	SubExpired  SubscriptionStatus = "expired"
)
