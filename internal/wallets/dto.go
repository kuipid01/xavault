package wallets

type InitiateDepositRequest struct {
	Amount         int64  `json:"amount" validate:"required,gt=0"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

type InitiateDepositResponse struct {
	AuthorizationURL string      `json:"authorization_url,omitempty"`
	AccessCode       string      `json:"access_code,omitempty"`
	Reference        string      `json:"reference"`
	ProviderResponse interface{} `json:"provider_response,omitempty"`
}

type DepositVerificationResponse struct {
	Status    string         `json:"status"`
	Reference string         `json:"reference"`
	Deposit   *WalletDeposit `json:"deposit,omitempty"`
}

const (
	WalletLedgerTypeWithdrawalHold     = "withdrawal_hold"
	WalletLedgerTypeWithdrawalReversal = "withdrawal_reversal"
	WalletLedgerTypeWithdrawalDebit    = "withdrawal_debit"
)
