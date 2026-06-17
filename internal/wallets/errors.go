package wallets

import "errors"

var (
	ErrWalletNotFound            = errors.New("wallet not found")
	ErrInvalidWalletAmount       = errors.New("invalid wallet amount")
	ErrInsufficientWalletBalance = errors.New("insufficient wallet balance")
	ErrInsufficientLockedBalance = errors.New("insufficient locked balance")
)
