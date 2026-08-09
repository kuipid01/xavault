package withdrawals

type PreviewWithdrawalRequest struct {
	SourceType string  `json:"source_type"`
	EnvelopeID *string `json:"envelope_id"`
	Amount     int64   `json:"amount"`
}

type RequestWithdrawalRequest struct {
	QuoteID         string  `json:"quote_id"`
	SourceType      string  `json:"source_type"`
	EnvelopeID      *string `json:"envelope_id"`
	Amount          int64   `json:"amount"`
	Reason          string  `json:"reason"`
	IdempotencyKey  string  `json:"idempotency_key" validate:"required"`
	DestinationType string  `json:"destination_type"`
	DestinationID   string  `json:"destination_id"`
	WithdrawalPin   string  `json:"withdrawal_pin"`
}

type WithdrawalResponse struct {
	ID                  string  `json:"id"`
	UserID              string  `json:"user_id"`
	WalletID            string  `json:"wallet_id"`
	PlanID              string  `json:"plan_id"`
	EnvelopeID          *string `json:"envelope_id"`
	SourceType          string  `json:"source_type"`
	Status              string  `json:"status"`
	Amount              int64   `json:"amount"`
	Currency            string  `json:"currency"`
	Reason              string  `json:"reason"`
	Reference           string  `json:"reference"`
	Description         string  `json:"description"`
	PaidAt              *string `json:"paid_at"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	ProviderTransferFee int64   `json:"provider_transfer_fee"`
	GovernmentLevy      int64   `json:"government_levy"`
	XavaultSubsidy      int64   `json:"xavault_subsidy"`
	UserFee             int64   `json:"user_fee"`
	TotalDeduction      int64   `json:"total_deduction"`
	RecipientAmount     int64   `json:"recipient_amount"`
	AllowanceApplied    bool    `json:"allowance_applied"`
}

type PreviewWithdrawalResponse struct {
	QuoteID             string   `json:"quote_id,omitempty"`
	ProviderTransferFee int64    `json:"provider_transfer_fee"`
	GovernmentLevy      int64    `json:"government_levy"`
	XavaultSubsidy      int64    `json:"xavault_subsidy"`
	UserFee             int64    `json:"user_fee"`
	TotalDeduction      int64    `json:"total_deduction"`
	RecipientAmount     int64    `json:"recipient_amount"`
	AllowanceApplied    bool     `json:"allowance_applied"`
	PricingVersion      string   `json:"pricing_version,omitempty"`
	ExpiresAt           string   `json:"expires_at,omitempty"`
	CanWithdraw         bool     `json:"can_withdraw"`
	SourceType          string   `json:"source_type"`
	PlanID              string   `json:"plan_id"`
	EnvelopeID          *string  `json:"envelope_id"`
	RequestedAmount     int64    `json:"requested_amount"`
	WithdrawableBalance int64    `json:"withdrawable_balance"`
	RemainingBalance    int64    `json:"remaining_balance"`
	ValidationErrors    []string `json:"validation_errors"`
}

type ListWithdrawalsResponse struct {
	Items []WithdrawalResponse `json:"items"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
	Total int64                `json:"total"`
}

type WalletSnapshotDTO struct {
	WalletID         string `json:"wallet_id"`
	AvailableBalance int64  `json:"available_balance"`
	LockedBalance    int64  `json:"locked_balance"`
	LedgerBalance    int64  `json:"ledger_balance"`
	Currency         string `json:"currency"`
}
