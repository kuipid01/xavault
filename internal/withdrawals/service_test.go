package withdrawals

import (
	"testing"

	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
)

func TestValidateRequestWithdrawalRequestRequiresDestination(t *testing.T) {
	errs := validateRequestWithdrawalRequest(RequestWithdrawalRequest{
		SourceType:     WithdrawalSourcePlanUnboxed,
		Amount:         1000,
		IdempotencyKey: "idem-1",
	})

	if errs["destination_type"] == "" {
		t.Fatalf("expected destination_type validation error")
	}

	if errs["destination_id"] == "" {
		t.Fatalf("expected destination_id validation error")
	}
}

func TestValidateRequestWithdrawalRequestAcceptsSavedAccountDestination(t *testing.T) {
	errs := validateRequestWithdrawalRequest(RequestWithdrawalRequest{
		SourceType:      WithdrawalSourcePlanUnboxed,
		Amount:          1000,
		IdempotencyKey:  "idem-1",
		DestinationType: WithdrawalDestinationAccount,
		DestinationID:   "account-1",
		WithdrawalPin:   "1234",
	})

	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %#v", errs)
	}
}

func TestValidateRequestWithdrawalRequestRejectsInvalidDestinationType(t *testing.T) {
	errs := validateRequestWithdrawalRequest(RequestWithdrawalRequest{
		SourceType:      WithdrawalSourcePlanUnboxed,
		Amount:          1000,
		IdempotencyKey:  "idem-1",
		DestinationType: "recipient_code",
		DestinationID:   "account-1",
		WithdrawalPin:   "1234",
	})

	if errs["destination_type"] == "" {
		t.Fatalf("expected invalid destination_type validation error")
	}
}

func TestValidateRequestWithdrawalRequestRequiresPin(t *testing.T) {
	errs := validateRequestWithdrawalRequest(RequestWithdrawalRequest{
		SourceType:      WithdrawalSourcePlanUnboxed,
		Amount:          1000,
		IdempotencyKey:  "idem-1",
		DestinationType: WithdrawalDestinationAccount,
		DestinationID:   "account-1",
	})

	if errs["withdrawal_pin"] == "" {
		t.Fatalf("expected withdrawal_pin validation error")
	}
}

func TestIsPlanWithdrawableStatus(t *testing.T) {
	allowedStatuses := []string{
		spendingplans.PlanStatusActive,
		spendingplans.PlanStatusExpired,
		spendingplans.PlanStatusCompleted,
	}

	for _, status := range allowedStatuses {
		if !isPlanWithdrawableStatus(status) {
			t.Fatalf("expected status %q to be withdrawable", status)
		}
	}

	blockedStatuses := []string{
		spendingplans.PlanStatusDraft,
		spendingplans.PlanStatusCancelled,
	}

	for _, status := range blockedStatuses {
		if isPlanWithdrawableStatus(status) {
			t.Fatalf("expected status %q to be blocked", status)
		}
	}
}
