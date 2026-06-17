package spendingplans

type EnvelopeAllocationReader interface {
	SumTopLevelEnvelopeAmount(planID string) (int64, error)
}
