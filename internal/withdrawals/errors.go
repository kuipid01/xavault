package withdrawals

import "errors"

var (
	ErrWithdrawalNotFound              = errors.New("withdrawal not found")
	ErrInvalidWithdrawalSource         = errors.New("invalid withdrawal source")
	ErrInvalidWithdrawalAmount         = errors.New("invalid withdrawal amount")
	ErrInsufficientWithdrawable        = errors.New("insufficient withdrawable balance")
	ErrEnvelopeRequired                = errors.New("envelope id is required")
	ErrEnvelopeNotAllowed              = errors.New("envelope is not allowed for this source type")
	ErrPlanNotActive                   = errors.New("plan is not active")
	ErrPlanNotFound                    = errors.New("plan not found")
	ErrEnvelopeNotFound                = errors.New("envelope not found")
	ErrWalletNotFound                  = errors.New("wallet not found")
	ErrSpendingPlanLockedUntilPlanEnds = errors.New("spending plan is locked until the plan ends")
	ErrPendingWithdrawalExists         = errors.New("pending withdrawal already exists")
)
