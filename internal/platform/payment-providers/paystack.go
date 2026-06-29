package paymentproviders

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kuipid01/salary-vault/internal/shared/client"
)

type PaystackProvider struct {
	apiKey         string
	baseUrl        string
	reusableClient *client.ReusableClient
}

func NewPaystackProvider(reusableClient *client.ReusableClient) *PaystackProvider {
	return &PaystackProvider{
		apiKey:         os.Getenv("PAYSTACK_SECRET_KEY"),
		baseUrl:        os.Getenv("PAYSTACK_BASE_URL"),
		reusableClient: reusableClient,
	}
}

func (p *PaystackProvider) InitiateDeposit(ctx context.Context, data *DepositBody) (*DepositResponse, error) {

	// 3. Define headers explicitly for Paystack
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}

	// 4. Properly initialize the target mad and p so it's not nil
	respTarget := make(map[string]interface{})

	// 5. Pass data directly (no manual json.Marshal here)
	url := p.baseUrl + "/transaction/initialize"
	err := p.reusableClient.DoRequest(ctx, http.MethodPost, url, headers, data, &respTarget)
	if err != nil {
		return nil, err
	}

	return &DepositResponse{
		Status:  true,
		Message: "Deposit initiated successfully",
		Data:    respTarget,
	}, nil
}

func (p *PaystackProvider) VerifyDeposit(ctx context.Context, reference string) (*DepositResponse, error) {
	// 3. Define headers explicitly for Paystack
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}

	// 4. Properly initialize the target map so it's not nil
	respTarget := make(map[string]interface{})

	// 5. Pass data directly (no manual json.Marshal here)
	url := p.baseUrl + "/transaction/verify/" + reference
	err := p.reusableClient.DoRequest(ctx, http.MethodGet, url, headers, nil, &respTarget)
	if err != nil {
		return nil, err
	}

	return &DepositResponse{
		Status:  true,
		Message: "Deposit verified successfully",
		Data:    respTarget,
	}, nil

}

func (p *PaystackProvider) ListBanks(ctx context.Context) (*ListBanksResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}

	var respTarget ListBanksResponse

	url := p.baseUrl + "/bank?country=nigeria&currency=NGN&type=nuban"
	err := p.reusableClient.DoRequest(ctx, http.MethodGet, url, headers, nil, &respTarget)
	if err != nil {
		return nil, err
	}

	if !respTarget.Status {
		return nil, fmt.Errorf("paystack bank list failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}

func (p *PaystackProvider) ResolveBankAccount(
	ctx context.Context,
	accountNumber string,
	bankCode string,
) (*ResolveBankAccountResponse, error) {
	start := time.Now()

	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}

	var respTarget ResolveBankAccountResponse

	params := url.Values{}
	params.Set("account_number", accountNumber)
	params.Set("bank_code", bankCode)

	requestURL := p.baseUrl + "/bank/resolve?" + params.Encode()

	log.Printf(
		"[PAYSTACK][RESOLVE_ACCOUNT][START] bank_code=%s account_number=%s url=%s",
		bankCode,
		maskAccountNumber(accountNumber),
		requestURL,
	)

	err := p.reusableClient.DoRequest(
		ctx,
		http.MethodGet,
		requestURL,
		headers,
		nil,
		&respTarget,
	)

	duration := time.Since(start)

	if err != nil {
		log.Printf(
			"[PAYSTACK][RESOLVE_ACCOUNT][ERROR] bank_code=%s account_number=%s duration=%s error=%v",
			bankCode,
			maskAccountNumber(accountNumber),
			duration,
			err,
		)

		return nil, fmt.Errorf("paystack resolve account request failed: %w", err)
	}

	log.Printf(
		"[PAYSTACK][RESOLVE_ACCOUNT][RESPONSE] bank_code=%s account_number=%s duration=%s status=%v message=%q account_name=%q",
		bankCode,
		maskAccountNumber(accountNumber),
		duration,
		respTarget.Status,
		respTarget.Message,
		respTarget.Data.AccountName,
	)

	if !respTarget.Status {
		log.Printf(
			"[PAYSTACK][RESOLVE_ACCOUNT][FAILED] bank_code=%s account_number=%s message=%q",
			bankCode,
			maskAccountNumber(accountNumber),
			respTarget.Message,
		)

		return nil, fmt.Errorf("paystack account resolve failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}
func maskAccountNumber(accountNumber string) string {
	if len(accountNumber) <= 4 {
		return "****"
	}

	return strings.Repeat("*", len(accountNumber)-4) + accountNumber[len(accountNumber)-4:]
}
func (p *PaystackProvider) CreateCustomer(
	ctx context.Context,
	data *CreateCustomerBody,
) (*CreateCustomerResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Content-Type":  "application/json",
	}

	var respTarget CreateCustomerResponse

	url := p.baseUrl + "/customer"
	err := p.reusableClient.DoRequest(ctx, http.MethodPost, url, headers, data, &respTarget)
	if err != nil {
		return nil, err
	}

	if !respTarget.Status {
		return nil, fmt.Errorf("paystack customer creation failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}

func (p *PaystackProvider) CreateDedicatedAccount(
	ctx context.Context,
	data *CreateDedicatedAccountBody,
) (*CreateDedicatedAccountResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Content-Type":  "application/json",
	}

	var respTarget CreateDedicatedAccountResponse

	url := p.baseUrl + "/dedicated_account"
	err := p.reusableClient.DoRequest(ctx, http.MethodPost, url, headers, data, &respTarget)
	if err != nil {
		return nil, err
	}

	if !respTarget.Status {
		return nil, fmt.Errorf("paystack dedicated account creation failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}

func (p *PaystackProvider) CreateTransferRecipient(
	ctx context.Context,
	data *CreateTransferRecipientBody,
) (*CreateTransferRecipientResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Content-Type":  "application/json",
	}

	var respTarget CreateTransferRecipientResponse

	url := p.baseUrl + "/transferrecipient"

	err := p.reusableClient.DoRequest(
		ctx,
		http.MethodPost,
		url,
		headers,
		data,
		&respTarget,
	)
	if err != nil {
		return nil, err
	}

	if !respTarget.Status {
		return nil, fmt.Errorf("paystack recipient creation failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}

// Data flow:
// 1. Backend has already created local withdrawal record.
// 2. Backend has already locked user funds locally.
// 3. This function only asks Paystack to start external transfer.
// 4. Final success should still come from webhook, not this response.
func (p *PaystackProvider) InitiateTransfer(
	ctx context.Context,
	data *InitiateTransferBody,
) (*InitiateTransferResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Content-Type":  "application/json",
	}

	var respTarget InitiateTransferResponse

	url := p.baseUrl + "/transfer"

	err := p.reusableClient.DoRequest(
		ctx,
		http.MethodPost,
		url,
		headers,
		data,
		&respTarget,
	)
	if err != nil {
		return nil, err
	}

	if !respTarget.Status {
		return nil, fmt.Errorf("paystack transfer failed: %s", respTarget.Message)
	}

	return &respTarget, nil
}

func (p *PaystackProvider) VerifyWithdrawal(ctx context.Context, reference string) (*WithdrawalResponse, error) {
	// 3. Define headers explicitly for Paystack
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}

	// 4. Properly initialize the target map so it's not nil
	respTarget := make(map[string]interface{})

	// 5. Pass data directly (no manual json.Marshal here)
	url := p.baseUrl + "/transfer/" + reference
	err := p.reusableClient.DoRequest(ctx, http.MethodGet, url, headers, nil, &respTarget)
	if err != nil {
		return nil, err
	}

	return &WithdrawalResponse{
		Status:  true,
		Message: "Withdrawal verified successfully",
		Data:    respTarget,
	}, nil
}
