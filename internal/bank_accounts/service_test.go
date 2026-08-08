package bankaccounts

import (
	"context"
	"testing"

	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
)

type fakePaymentProvider struct {
	resolvedAccountNumber string
	resolvedBankCode      string
}

func (f *fakePaymentProvider) InitiateDeposit(ctx context.Context, body *paymentproviders.DepositBody) (*paymentproviders.DepositResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) VerifyDeposit(ctx context.Context, reference string) (*paymentproviders.VerifyTransactionResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) ListBanks(ctx context.Context) (*paymentproviders.ListBanksResponse, error) {
	return &paymentproviders.ListBanksResponse{
		Status: true,
		Data: []paymentproviders.PaystackBank{
			{Name: "Test Bank", Code: "001", Currency: "NGN"},
		},
	}, nil
}

func (f *fakePaymentProvider) ResolveBankAccount(ctx context.Context, accountNumber string, bankCode string) (*paymentproviders.ResolveBankAccountResponse, error) {
	f.resolvedAccountNumber = accountNumber
	f.resolvedBankCode = bankCode

	return &paymentproviders.ResolveBankAccountResponse{
		Status: true,
		Data: paymentproviders.ResolveBankAccountData{
			AccountNumber: accountNumber,
			AccountName:   "Ada Lovelace",
		},
	}, nil
}

func (f *fakePaymentProvider) CreateCustomer(ctx context.Context, body *paymentproviders.CreateCustomerBody) (*paymentproviders.CreateCustomerResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) CreateDedicatedAccount(ctx context.Context, body *paymentproviders.CreateDedicatedAccountBody) (*paymentproviders.CreateDedicatedAccountResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) CreateTransferRecipient(ctx context.Context, body *paymentproviders.CreateTransferRecipientBody) (*paymentproviders.CreateTransferRecipientResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) InitiateTransfer(ctx context.Context, body *paymentproviders.InitiateTransferBody) (*paymentproviders.InitiateTransferResponse, error) {
	return nil, nil
}

func (f *fakePaymentProvider) VerifyWithdrawal(ctx context.Context, reference string) (*paymentproviders.VerifyTransferResponse, error) {
	return nil, nil
}

func TestResolveAccountTrimsInputAndReturnsResolvedName(t *testing.T) {
	payment := &fakePaymentProvider{}
	svc := NewService(nil, payment)

	result := svc.ResolveAccount(context.Background(), ResolveBankAccountRequest{
		BankCode:      " 001 ",
		AccountNumber: " 0123456789 ",
	})

	if result.Error != nil {
		t.Fatalf("expected successful resolve, got %v", result.Error)
	}

	if payment.resolvedBankCode != "001" {
		t.Fatalf("expected trimmed bank code, got %q", payment.resolvedBankCode)
	}

	if payment.resolvedAccountNumber != "0123456789" {
		t.Fatalf("expected trimmed account number, got %q", payment.resolvedAccountNumber)
	}

	data, ok := result.Data.(ResolveBankAccountResponse)
	if !ok {
		t.Fatalf("expected ResolveBankAccountResponse, got %T", result.Data)
	}

	if data.AccountName != "Ada Lovelace" {
		t.Fatalf("expected resolved account name, got %q", data.AccountName)
	}
}

func TestResolveAccountRequiresBankAndAccountNumber(t *testing.T) {
	svc := NewService(nil, &fakePaymentProvider{})

	result := svc.ResolveAccount(context.Background(), ResolveBankAccountRequest{})

	if result.Error == nil {
		t.Fatalf("expected validation error")
	}
}
