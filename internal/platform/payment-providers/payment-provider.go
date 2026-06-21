package paymentproviders

import "context"

type PaymentProvider interface {
	InitiateDeposit(ctx context.Context, body *DepositBody) (*DepositResponse, error)
	VerifyDeposit(ctx context.Context, reference string) (*DepositResponse, error)
	InitiateTransfer(ctx context.Context, body *InitiateTransferBody) (*InitiateTransferResponse, error)
	VerifyWithdrawal(ctx context.Context, reference string) (*WithdrawalResponse, error)
}

const (
	Paystack = "Paystack"
)
