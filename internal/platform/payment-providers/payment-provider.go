package paymentproviders

import "context"

type PaymentProvider interface {
	InitiateDeposit(ctx context.Context, body *DepositBody) (*DepositResponse, error)
	VerifyDeposit(reference string) (*DepositResponse, error)
	InitiateWithdrawal(amount int64, currency string) (*WithdrawalResponse, error)
	VerifyWithdrawal(reference string) (*WithdrawalResponse, error)
}
