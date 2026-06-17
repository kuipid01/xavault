package withdrawals

import "time"

func mapWithdrawalToResponse(withdrawal *Withdrawal) WithdrawalResponse {
	if withdrawal == nil {
		return WithdrawalResponse{}
	}

	return WithdrawalResponse{
		ID:          withdrawal.ID,
		UserID:      withdrawal.UserID,
		WalletID:    withdrawal.WalletID,
		PlanID:      withdrawal.PlanID,
		EnvelopeID:  withdrawal.EnvelopeID,
		SourceType:  withdrawal.SourceType,
		Status:      withdrawal.Status,
		Amount:      withdrawal.Amount,
		Currency:    withdrawal.Currency,
		Reason:      withdrawal.Reason,
		Reference:   withdrawal.Reference,
		Description: withdrawal.Description,
		PaidAt:      formatOptionalTime(withdrawal.PaidAt),
		CreatedAt:   formatTime(withdrawal.CreatedAt),
		UpdatedAt:   formatTime(withdrawal.UpdatedAt),
	}
}

func mapWithdrawalsToResponse(withdrawals []Withdrawal) []WithdrawalResponse {
	responses := make([]WithdrawalResponse, 0, len(withdrawals))

	for _, withdrawal := range withdrawals {
		responses = append(responses, mapWithdrawalToResponse(&withdrawal))
	}

	return responses
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format(time.RFC3339)
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}

	formatted := value.Format(time.RFC3339)
	return &formatted
}
