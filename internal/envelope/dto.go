package envelope

type CreateEnvelopeRequest struct {
	Name                 string  `json:"name"`
	Amount               int64   `json:"amount"`
	StartDate            string  `json:"start_date"`
	EndDate              string  `json:"end_date"`
	ParentEnvelopeID     *string `json:"parent_envelope_id"`
	UnboxedBalancePolicy string  `json:"unboxed_balance_policy"`
}

type UpdateEnvelopeRequest struct {
	Name                 *string `json:"name"`
	Amount               *int64  `json:"amount"`
	StartDate            *string `json:"start_date"`
	EndDate              *string `json:"end_date"`
	UnboxedBalancePolicy *string `json:"unboxed_balance_policy"`
}

type EnvelopeResponse struct {
	ID                      string             `json:"id"`
	UserID                  string             `json:"user_id"`
	PlanID                  string             `json:"plan_id"`
	ParentEnvelopeID        *string            `json:"parent_envelope_id"`
	Name                    string             `json:"name"`
	Amount                  int64              `json:"amount"`
	Status                  string             `json:"status"`
	UnboxedBalancePolicy    string             `json:"unboxed_balance_policy"`
	StartDate               string             `json:"start_date"`
	EndDate                 string             `json:"end_date"`
	AllocatedToChildren     int64              `json:"allocated_to_children"`
	UnboxedBalance          int64              `json:"unboxed_balance"`
	WithdrawnAmount         int64              `json:"withdrawn_amount"`
	PendingWithdrawalAmount int64              `json:"pending_withdrawal_amount"`
	Children                []EnvelopeResponse `json:"children,omitempty"`
	CreatedAt               string             `json:"created_at"`
	UpdatedAt               string             `json:"updated_at"`
}
