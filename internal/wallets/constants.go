package wallets

const (
	WalletLedgerTypeDebit                 = "debit"
	WalletLedgerTypeCredit                = "credit"
	WalletLedgerTypeLock                  = "lock"
	WalletLedgerTypeUnlock                = "unlock"
	WalletLedgerTypeRefund                = "refund"
	WalletLedgerTypeCancel                = "cancel"
	WalletLedgerSourceSpendingPlan        = "spending_plan"
	WalletLedgerSourcePlanCancellationFee = "plan_cancellation_fee"
	WalletLedgerSourceWithdrawal          = "withdrawal"
	WalletStatusPending                   = "pending"
	DebitSourceSpendingWallet             = "spending_wallet"
	DebitSourcePlan                       = "spending_wallet"
	DebitSourceEnvelope                   = "envelope"
	DebitSourceWithdrawal                 = "withdrawal"
	DebitSourcePlanCancellationFee        = "plan_cancellation_fee"
	WalletLedgerTypeWithdrawal            = "withdrawal"
)
