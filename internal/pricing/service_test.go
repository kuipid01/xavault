package pricing

import (
	"strings"
	"testing"
)

func TestPricingTableNamesMatchVersionedMigration(t *testing.T) {
	if got := (FeeRule{}).TableName(); got != "provider_fee_rules" {
		t.Fatalf("fee rule table=%q", got)
	}
	if got := (Quote{}).TableName(); got != "fee_quotes" {
		t.Fatalf("quote table=%q", got)
	}
}

func TestFeeBoundaries(t *testing.T) {
	threshold := int64(250000)
	capFee := int64(200000)
	r := FeeRule{PercentageBasisPoints: 150, FixedFeeKobo: 10000, FeeWaiverThresholdKobo: &threshold, MaximumFeeKobo: &capFee}
	if got := feeFor(r, 249900); got != 3749 {
		t.Fatalf("below waiver fee=%d", got)
	}
	if got := feeFor(r, 250000); got != 13750 {
		t.Fatalf("threshold fee=%d", got)
	}
	if got := feeFor(r, 20_000_000); got != 200000 {
		t.Fatalf("cap fee=%d", got)
	}
}

func TestWithdrawalAmountsDeductFeesFromRequestedAmount(t *testing.T) {
	userFee, subsidy, totalDeduction, recipientAmount, err := withdrawalAmounts(1_000_000, 2_500, 5_000, false)
	if err != nil {
		t.Fatal(err)
	}
	if userFee != 7_500 || subsidy != 0 || totalDeduction != 1_000_000 || recipientAmount != 992_500 {
		t.Fatalf("got userFee=%d subsidy=%d totalDeduction=%d recipientAmount=%d", userFee, subsidy, totalDeduction, recipientAmount)
	}
}

func TestWithdrawalAmountsSubsidyPreservesRecipientAmount(t *testing.T) {
	userFee, subsidy, totalDeduction, recipientAmount, err := withdrawalAmounts(1_000_000, 2_500, 5_000, true)
	if err != nil {
		t.Fatal(err)
	}
	if userFee != 0 || subsidy != 7_500 || totalDeduction != 1_000_000 || recipientAmount != 1_000_000 {
		t.Fatalf("got userFee=%d subsidy=%d totalDeduction=%d recipientAmount=%d", userFee, subsidy, totalDeduction, recipientAmount)
	}
}

func TestWithdrawalAmountsRejectFeesThatConsumeAmount(t *testing.T) {
	_, _, _, _, err := withdrawalAmounts(1_000, 600, 400, false)
	if err == nil || !strings.Contains(err.Error(), "must exceed applicable fees") {
		t.Fatalf("expected fee validation error, got %v", err)
	}
}
