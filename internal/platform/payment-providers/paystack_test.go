package paymentproviders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kuipid01/salary-vault/internal/shared/client"
)

func TestVerifyDepositUsesTypedNestedData(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"Verification successful","data":{"id":91,"reference":"dep-1","status":"success","amount":250000,"currency":"NGN"}}`))
	}))
	defer server.Close()

	provider := &PaystackProvider{apiKey: "test", baseUrl: server.URL, reusableClient: client.NewReusableClient(time.Second)}
	result, err := provider.VerifyDeposit(context.Background(), "dep-1")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/transaction/verify/dep-1" {
		t.Fatalf("unexpected verification path %q", path)
	}
	if result.Data.Reference != "dep-1" || result.Data.Amount != 250000 || result.Data.Status != "success" {
		t.Fatalf("unexpected typed verification data: %+v", result.Data)
	}
}

func TestVerifyWithdrawalUsesTransferVerifyEndpoint(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"Transfer retrieved","data":{"id":42,"reference":"wd-1","transfer_code":"TRF_1","status":"success","amount":100000,"currency":"NGN"}}`))
	}))
	defer server.Close()

	provider := &PaystackProvider{apiKey: "test", baseUrl: server.URL, reusableClient: client.NewReusableClient(time.Second)}
	result, err := provider.VerifyWithdrawal(context.Background(), "wd-1")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/transfer/verify/wd-1" {
		t.Fatalf("unexpected verification path %q", path)
	}
	if result.Data.Reference != "wd-1" || result.Data.TransferCode != "TRF_1" || result.Data.Status != "success" {
		t.Fatalf("unexpected typed transfer data: %+v", result.Data)
	}
}
