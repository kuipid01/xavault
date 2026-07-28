package spendingplans

import "errors"

var (
	ErrPlanNotFound              = errors.New("plan not found")
	ErrPlanNotDraft              = errors.New("only draft plans can be updated")
	ErrPlanNotActive             = errors.New("plan is not active")
	ErrPlanAlreadyFinal          = errors.New("plan is already completed, cancelled, or expired")
	ErrPlanAlreadyFunded         = errors.New("plan is already funded")
	ErrInvalidPlanAmount         = errors.New("invalid plan amount")
	ErrInvalidPlanDateRange      = errors.New("invalid plan date range")
	ErrPlanLockedCannotCancel    = errors.New("this plan is locked and cannot be cancelled")
	ErrPlanNotFunded             = errors.New("this plan has not been funded")
	ErrInsufficientLockedBalance = errors.New("insufficient locked balance")
	ErrPlanCannotCancel          = errors.New("this plan cannot be cancelled")
	ErrPlanWithdrawalInProgress  = errors.New("this plan has a withdrawal in progress")
)
