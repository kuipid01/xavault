package spendingplans

const (
	PlanFundingStatusUnfunded  = "unfunded"
	PlanFundingStatusLocked    = "locked"
	PlanFundingStatusReleased  = "released"
	PlanCancellationFeePercent = int64(3)
	PlanCancellationFeeCap     = int64(500000) // ₦5,000 in kobo

)
