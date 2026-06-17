package envelope

import "errors"

var (
	ErrPlanNotFound                   = errors.New("plan not found")
	ErrPlanNotDraft                   = errors.New("plan is not in draft status")
	ErrEnvelopeNotFound               = errors.New("envelope not found")
	ErrEnvelopeParentNotFound         = errors.New("parent envelope not found")
	ErrEnvelopeAmountExceeded         = errors.New("envelope amount exceeds available allocation")
	ErrEnvelopeInvalidDateRange       = errors.New("envelope date range is invalid")
	ErrEnvelopeCannotBeParentOfItself = errors.New("envelope cannot be parent of itself")
	ErrEnvelopeHasChildren            = errors.New("envelope has child envelopes")
	ErrEnvelopeCannotDelete           = errors.New("envelope cannot be deleted")
	ErrEnvelopeLockedCannotDelete     = errors.New("locked envelope cannot be deleted")
)
