package spendingplans

import (
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

func ValidateCreatePlanRequest(req CreatePlanRequest) map[string]string {
	errors := make(map[string]string)

	name := strings.TrimSpace(req.Name)
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	policy := strings.TrimSpace(req.UnboxedBalancePolicy)

	if name == "" {
		errors["name"] = "Name is required"
	} else if len(name) < 2 {
		errors["name"] = "Name must be at least 2 characters"
	} else if len(name) > 100 {
		errors["name"] = "Name must not be more than 100 characters"
	}

	if req.Amount <= 0 {
		errors["amount"] = "Amount must be greater than zero"
	}

	if currency == "" {
		errors["currency"] = "Currency is required"
	} else if currency != "NGN" {
		errors["currency"] = "Currency must be NGN"
	}

	if policy == "" {
		errors["unboxed_balance_policy"] = "Unboxed balance policy is required"
	} else if !IsValidPlanUnboxedPolicy(policy) {
		errors["unboxed_balance_policy"] = "Invalid unboxed balance policy"
	}

	startDate, startErr := ParseDate(req.StartDate)
	if strings.TrimSpace(req.StartDate) == "" {
		errors["start_date"] = "Start date is required"
	} else if startErr != nil {
		errors["start_date"] = "Start date must be in YYYY-MM-DD format"
	}

	endDate, endErr := ParseDate(req.EndDate)
	if strings.TrimSpace(req.EndDate) == "" {
		errors["end_date"] = "End date is required"
	} else if endErr != nil {
		errors["end_date"] = "End date must be in YYYY-MM-DD format"
	}

	if startErr == nil && endErr == nil {
		if endDate.Before(startDate) {
			errors["end_date"] = "End date must be after start date"
		}

		if endDate.Equal(startDate) {
			errors["end_date"] = "End date must be after start date"
		}
	}

	return errors
}

func ValidateUpdatePlanRequest(req UpdatePlanRequest) map[string]string {
	errors := make(map[string]string)

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)

		if name == "" {
			errors["name"] = "Name cannot be empty"
		} else if len(name) < 2 {
			errors["name"] = "Name must be at least 2 characters"
		} else if len(name) > 100 {
			errors["name"] = "Name must not be more than 100 characters"
		}
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			errors["amount"] = "Amount must be greater than zero"
		}
	}

	if req.UnboxedBalancePolicy != nil {
		policy := strings.TrimSpace(*req.UnboxedBalancePolicy)

		if policy == "" {
			errors["unboxed_balance_policy"] = "Unboxed balance policy cannot be empty"
		} else if !IsValidPlanUnboxedPolicy(policy) {
			errors["unboxed_balance_policy"] = "Invalid unboxed balance policy"
		}
	}

	var startDate *time.Time
	var endDate *time.Time

	if req.StartDate != nil {
		parsedStartDate, err := ParseDate(*req.StartDate)
		if err != nil {
			errors["start_date"] = "Start date must be in YYYY-MM-DD format"
		} else {
			startDate = &parsedStartDate
		}
	}

	if req.EndDate != nil {
		parsedEndDate, err := ParseDate(*req.EndDate)
		if err != nil {
			errors["end_date"] = "End date must be in YYYY-MM-DD format"
		} else {
			endDate = &parsedEndDate
		}
	}

	if startDate != nil && endDate != nil {
		if endDate.Before(*startDate) || endDate.Equal(*startDate) {
			errors["end_date"] = "End date must be after start date"
		}
	}

	return errors
}

func ValidateConfirmPlanRequest(req ConfirmPlanRequest) map[string]string {
	errors := make(map[string]string)

	if !req.AcceptedPreview {
		errors["accepted_preview"] = "Accepted preview is required"
	}

	return errors
}

func ValidateCancelPlanRequest(req CancelPlanRequest) map[string]string {
	errors := make(map[string]string)

	reason := strings.TrimSpace(req.Reason)

	if len(reason) > 255 {
		errors["reason"] = "Reason must not be more than 255 characters"
	}

	return errors
}

func ParseDate(value string) (time.Time, error) {
	return time.Parse(DateLayout, strings.TrimSpace(value))
}

func IsValidPlanUnboxedPolicy(policy string) bool {
	switch strings.TrimSpace(policy) {
	case PlanUnboxedPolicyFlexible, PlanUnboxedPolicyLockedUntilPlanEnds:
		return true
	default:
		return false
	}
}

func IsValidPlanStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PlanStatusDraft,
		PlanStatusActive,
		PlanStatusCompleted,
		PlanStatusCancelled,
		PlanStatusExpired:
		return true
	default:
		return false
	}
}
