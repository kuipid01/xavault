package wallets

type InitiateDepositRequest struct {
	Amount         int64  `json:"amount" validate:"required,gt=0"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

const (
	WalletLedgerTypeWithdrawalHold     = "withdrawal_hold"
	WalletLedgerTypeWithdrawalReversal = "withdrawal_reversal"
	WalletLedgerTypeWithdrawalDebit    = "withdrawal_debit"
)
