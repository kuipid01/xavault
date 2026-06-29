package paymentproviders

import (
	"context"
	"fmt"
	"net/http"
	"os"

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
