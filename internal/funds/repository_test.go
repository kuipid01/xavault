package funds

import "testing"

func TestAvailableBalanceSubtractsAllocationsAndCommittedWithdrawals(t *testing.T) {
	totals := WithdrawalTotals{Paid: 2_000, Pending: 1_000}
	if got := AvailableBalance(10_000, 4_000, totals); got != 3_000 {
		t.Fatalf("expected 3000, got %d", got)
	}
}

func TestAvailableBalanceNeverReturnsNegative(t *testing.T) {
	if got := AvailableBalance(1_000, 800, WithdrawalTotals{Paid: 500}); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}
