package spendingplans

import "time"

func mapPlanToResponse(plan *SpendingPlan, topLevelAllocated int64) PlanResponse {
	if plan == nil {
		return PlanResponse{}
	}

	return PlanResponse{
		ID:                   plan.ID,
		UserID:               plan.UserID,
		WalletID:             plan.WalletID,
		Name:                 plan.Name,
		Amount:               plan.Amount,
		Currency:             plan.Currency,
		Status:               plan.Status,
		UnboxedBalancePolicy: plan.UnboxedBalancePolicy,
		StartDate:            formatDate(plan.StartDate),
		EndDate:              formatDate(plan.EndDate),
		TopLevelAllocated:    topLevelAllocated,
		PlanUnboxedBalance:   plan.Amount - topLevelAllocated,
		ConfirmedAt:          formatOptionalTime(plan.ConfirmedAt),
		CreatedAt:            formatTime(plan.CreatedAt),
		UpdatedAt:            formatTime(plan.UpdatedAt),
	}
}

func mapPlansToResponse(plans []SpendingPlan) []PlanResponse {
	responses := make([]PlanResponse, 0, len(plans))

	for _, plan := range plans {
		responses = append(responses, mapPlanToResponse(&plan, 0))
	}

	return responses
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format("2006-01-02")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format(time.RFC3339)
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}

	formatted := value.Format(time.RFC3339)
	return &formatted
}
