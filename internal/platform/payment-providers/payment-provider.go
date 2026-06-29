package paymentproviders

import "context"

type PaymentProvider interface {
	InitiateDeposit(ctx context.Context, body *DepositBody) (*DepositResponse, error)
	VerifyDeposit(ctx context.Context, reference string) (*DepositResponse, error)
	ListBanks(ctx context.Context) (*ListBanksResponse, error)
	ResolveBankAccount(ctx context.Context, accountNumber string, bankCode string) (*ResolveBankAccountResponse, error)
	CreateCustomer(ctx context.Context, body *CreateCustomerBody) (*CreateCustomerResponse, error)
	CreateDedicatedAccount(ctx context.Context, body *CreateDedicatedAccountBody) (*CreateDedicatedAccountResponse, error)
	CreateTransferRecipient(ctx context.Context, body *CreateTransferRecipientBody) (*CreateTransferRecipientResponse, error)
	InitiateTransfer(ctx context.Context, body *InitiateTransferBody) (*InitiateTransferResponse, error)
	VerifyWithdrawal(ctx context.Context, reference string) (*WithdrawalResponse, error)
}

const (
	Paystack = "Paystack"
)
