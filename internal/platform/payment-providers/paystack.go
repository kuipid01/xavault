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

	// 4. Properly initialize the target map so it's not nil
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
	fmt.Println(respTarget, "HEY")
	return &DepositResponse{
		Status:  true,
		Message: "Deposit verified successfully",
		Data:    respTarget,
	}, nil

}

func (p *PaystackProvider) InitiateWithdrawal(amount int64, currency string) (*WithdrawalResponse, error) {
	return &WithdrawalResponse{}, nil
}

func (p *PaystackProvider) VerifyWithdrawal(reference string) (*WithdrawalResponse, error) {
	return &WithdrawalResponse{}, nil
}
