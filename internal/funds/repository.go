package funds

import "gorm.io/gorm"

type WithdrawalTotals struct {
	Paid    int64 `json:"withdrawn_amount"`
	Pending int64 `json:"pending_withdrawal_amount"`
}

func (t WithdrawalTotals) Committed() int64 { return t.Paid + t.Pending }

func AvailableBalance(amount, allocated int64, totals WithdrawalTotals) int64 {
	available := amount - allocated - totals.Committed()
	if available < 0 {
		return 0
	}
	return available
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) dbOrTx(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *Repository) totals(tx *gorm.DB, query string, args ...interface{}) (WithdrawalTotals, error) {
	var totals WithdrawalTotals
	err := r.dbOrTx(tx).Table("withdrawals").
		Select(`
			COALESCE(SUM(CASE WHEN status IN ('paid', 'success') THEN COALESCE(total_deduction, amount) ELSE 0 END), 0) AS paid,
			COALESCE(SUM(CASE WHEN status IN ('pending', 'processing') THEN COALESCE(total_deduction, amount) ELSE 0 END), 0) AS pending
		`).
		Where("deleted_at IS NULL").
		Where(query, args...).
		Scan(&totals).Error
	return totals, err
}

func (r *Repository) PlanUnboxedTotals(tx *gorm.DB, planID, userID string) (WithdrawalTotals, error) {
	return r.totals(tx, "plan_id = ? AND user_id = ? AND source_type = ?", planID, userID, "plan_unboxed")
}

func (r *Repository) EnvelopeTotals(tx *gorm.DB, envelopeID, userID string) (WithdrawalTotals, error) {
	return r.totals(tx, "envelope_id = ? AND user_id = ? AND source_type = ?", envelopeID, userID, "envelope")
}

func (r *Repository) EnvelopeHasWithdrawalHistory(tx *gorm.DB, envelopeID, userID string) (bool, error) {
	var count int64
	err := r.dbOrTx(tx).Table("withdrawals").
		Where("envelope_id = ? AND user_id = ? AND source_type = ? AND deleted_at IS NULL", envelopeID, userID, "envelope").
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) PlanTotals(tx *gorm.DB, planID, userID string) (WithdrawalTotals, error) {
	return r.totals(tx, "plan_id = ? AND user_id = ?", planID, userID)
}

func (r *Repository) EnvelopeTotalsByPlan(planID, userID string) (map[string]WithdrawalTotals, error) {
	type row struct {
		EnvelopeID string
		Paid       int64
		Pending    int64
	}
	var rows []row
	err := r.db.Table("withdrawals").
		Select(`envelope_id,
			COALESCE(SUM(CASE WHEN status IN ('paid', 'success') THEN COALESCE(total_deduction, amount) ELSE 0 END), 0) AS paid,
			COALESCE(SUM(CASE WHEN status IN ('pending', 'processing') THEN COALESCE(total_deduction, amount) ELSE 0 END), 0) AS pending`).
		Where("plan_id = ? AND user_id = ? AND source_type = ? AND envelope_id IS NOT NULL AND deleted_at IS NULL", planID, userID, "envelope").
		Group("envelope_id").Scan(&rows).Error
	result := make(map[string]WithdrawalTotals, len(rows))
	for _, item := range rows {
		result[item.EnvelopeID] = WithdrawalTotals{Paid: item.Paid, Pending: item.Pending}
	}
	return result, err
}
