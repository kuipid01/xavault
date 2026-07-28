package spendingplans

type CreatePlanRequest struct {
	Name                 string `json:"name" validate:"required,min=2,max=100"`
	Amount               int64  `json:"amount" validate:"required,gt=0"`
	Currency             string `json:"currency" validate:"omitempty,oneof=NGN"`
	StartDate            string `json:"start_date" validate:"required"`
	EndDate              string `json:"end_date" validate:"required"`
	UnboxedBalancePolicy string `json:"unboxed_balance_policy" validate:"required,oneof=flexible locked_until_plan_ends"`
}

type UpdatePlanRequest struct {
	Name                 *string `json:"name" validate:"omitempty,min=2,max=100"`
	Amount               *int64  `json:"amount" validate:"omitempty,gt=0"`
	StartDate            *string `json:"start_date" validate:"omitempty"`
	EndDate              *string `json:"end_date" validate:"omitempty"`
	UnboxedBalancePolicy *string `json:"unboxed_balance_policy" validate:"omitempty,oneof=flexible locked_until_plan_ends"`
}

type ConfirmPlanRequest struct {
	AcceptedPreview bool `json:"accepted_preview" validate:"required"`
}

type CancelPlanRequest struct {
	Reason string `json:"reason" validate:"omitempty,max=255"`
}

type ListPlansQuery struct {
	Status string `query:"status" validate:"omitempty,oneof=draft active completed cancelled expired"`
	Page   int    `query:"page" validate:"omitempty,min=1"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
}

type PlanResponse struct {
	ID                      string  `json:"id"`
	UserID                  string  `json:"user_id"`
	WalletID                string  `json:"wallet_id"`
	Name                    string  `json:"name"`
	Amount                  int64   `json:"amount"`
	Currency                string  `json:"currency"`
	Status                  string  `json:"status"`
	UnboxedBalancePolicy    string  `json:"unboxed_balance_policy"`
	StartDate               string  `json:"start_date"`
	EndDate                 string  `json:"end_date"`
	TopLevelAllocated       int64   `json:"top_level_allocated"`
	PlanUnboxedBalance      int64   `json:"plan_unboxed_balance"`
	WithdrawnAmount         int64   `json:"withdrawn_amount"`
	PendingWithdrawalAmount int64   `json:"pending_withdrawal_amount"`
	ConfirmedAt             *string `json:"confirmed_at"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

type ListPlansResponse struct {
	Items      []PlanResponse `json:"items"`
	Pagination PaginationDTO  `json:"pagination"`
}

type PaginationDTO struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type PreviewPlanResponse struct {
	CanConfirm         bool                  `json:"can_confirm"`
	PlanID             string                `json:"plan_id"`
	PlanAmount         int64                 `json:"plan_amount"`
	WalletAvailable    int64                 `json:"wallet_available"`
	TopLevelAllocated  int64                 `json:"top_level_allocated"`
	PlanUnboxedBalance int64                 `json:"plan_unboxed_balance"`
	UnboxedPolicy      string                `json:"unboxed_policy"`
	ValidationErrors   []PlanValidationError `json:"validation_errors"`
	EnvelopeTree       []EnvelopePreviewNode `json:"envelope_tree"`
}

type PlanValidationError struct {
	Code     string  `json:"code"`
	Message  string  `json:"message"`
	Field    string  `json:"field,omitempty"`
	EntityID *string `json:"entity_id,omitempty"`
	Entity   string  `json:"entity,omitempty"`
	Shortage int64   `json:"shortage,omitempty"`
}

type EnvelopePreviewNode struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	Amount               int64                 `json:"amount"`
	AllocatedToChildren  int64                 `json:"allocated_to_children"`
	UnboxedBalance       int64                 `json:"unboxed_balance"`
	UnboxedBalancePolicy string                `json:"unboxed_balance_policy"`
	StartDate            string                `json:"start_date"`
	EndDate              string                `json:"end_date"`
	Status               string                `json:"status"`
	Children             []EnvelopePreviewNode `json:"children"`
}

type ConfirmPlanResponse struct {
	Plan        PlanResponse      `json:"plan"`
	Wallet      WalletSnapshotDTO `json:"wallet"`
	LedgerEntry LedgerSnapshotDTO `json:"ledger_entry"`
}

type CancelPlanResponse struct {
	Plan            PlanResponse       `json:"plan"`
	Wallet          *WalletSnapshotDTO `json:"wallet,omitempty"`
	Message         string             `json:"message"`
	CancellationFee int64              `json:"cancellation_fee"`
	RefundedAmount  int64              `json:"refunded_amount"`
}

type WalletSnapshotDTO struct {
	WalletID         string `json:"wallet_id"`
	AvailableBalance int64  `json:"available_balance"`
	LockedBalance    int64  `json:"locked_balance"`
	LedgerBalance    int64  `json:"ledger_balance"`
	Currency         string `json:"currency"`
}

type LedgerSnapshotDTO struct {
	Reference     string `json:"reference"`
	Type          string `json:"type"`
	Source        string `json:"source"`
	Amount        int64  `json:"amount"`
	BalanceBefore int64  `json:"balance_before"`
	BalanceAfter  int64  `json:"balance_after"`
	Description   string `json:"description"`
}
