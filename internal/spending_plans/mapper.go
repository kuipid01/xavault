package spendingplans

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/funds"
)

// mapPlanToResponse builds the plan API response. unboxedTotals feeds the
// plan_unboxed_balance math (only plan_unboxed withdrawals), while allTotals
// carries every withdrawal against the plan (envelope + plan_unboxed) so the
// client can show how much of the plan has been paid out overall.
func mapPlanToResponse(plan *SpendingPlan, topLevelAllocated int64, unboxedTotals funds.WithdrawalTotals, allTotals funds.WithdrawalTotals) PlanResponse {
	if plan == nil {
		return PlanResponse{}
	}

	remainingBalance := max(plan.Amount-allTotals.Committed(), 0)

	return PlanResponse{
		ID:                      plan.ID,
		UserID:                  plan.UserID,
		WalletID:                plan.WalletID,
		Name:                    plan.Name,
		Amount:                  plan.Amount,
		Currency:                plan.Currency,
		Status:                  plan.Status,
		UnboxedBalancePolicy:    plan.UnboxedBalancePolicy,
		StartDate:               formatDate(plan.StartDate),
		EndDate:                 formatDate(plan.EndDate),
		TopLevelAllocated:       topLevelAllocated,
		PlanUnboxedBalance:      funds.AvailableBalance(plan.Amount, topLevelAllocated, unboxedTotals),
		WithdrawnAmount:         allTotals.Paid,
		PendingWithdrawalAmount: allTotals.Pending,
		RemainingBalance:        remainingBalance,
		IsExhausted:             remainingBalance <= 0,
		ConfirmedAt:             formatOptionalTime(plan.ConfirmedAt),
		CreatedAt:               formatTime(plan.CreatedAt),
		UpdatedAt:               formatTime(plan.UpdatedAt),
	}
}

func mapPlansToResponse(plans []SpendingPlan) []PlanResponse {
	responses := make([]PlanResponse, 0, len(plans))

	for _, plan := range plans {
		responses = append(responses, mapPlanToResponse(&plan, 0, funds.WithdrawalTotals{}, funds.WithdrawalTotals{}))
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
