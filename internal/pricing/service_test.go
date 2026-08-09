package pricing

import "testing"

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
