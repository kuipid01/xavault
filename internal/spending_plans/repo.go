package spendingplans

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

// Create creates a new spending plan.
func (r *PlanRepository) Create(plan *SpendingPlan) error {
	return r.db.Create(plan).Error
}

// FindByID finds a plan by plan ID.
func (r *PlanRepository) FindByID(planID string) (*SpendingPlan, error) {
	var plan SpendingPlan

	err := r.db.
		Where("id = ?", planID).
		First(&plan).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &plan, nil
}

// FindByIDAndUserID finds a plan owned by a specific user.
func (r *PlanRepository) FindByIDAndUserID(planID string, userID string) (*SpendingPlan, error) {
	var plan SpendingPlan

	err := r.db.
		Where("id = ? AND user_id = ?", planID, userID).
		First(&plan).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &plan, nil
}

// FindByIDAndUserIDForUpdate locks a plan row inside a transaction.
// Use this when confirming/cancelling/updating money-sensitive plan state.
func (r *PlanRepository) FindByIDAndUserIDForUpdate(tx *gorm.DB, planID string, userID string) (*SpendingPlan, error) {
	var plan SpendingPlan

	err := tx.
		Set("gorm:query_option", "FOR UPDATE").
		Where("id = ? AND user_id = ?", planID, userID).
		First(&plan).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &plan, nil
}

// FindAllByUserID returns user's plans with optional status filter and pagination.
func (r *PlanRepository) FindAllByUserID(userID string, status string, page int, limit int) ([]SpendingPlan, int64, error) {
	var plans []SpendingPlan
	var total int64

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	query := r.db.Model(&SpendingPlan{}).Where("user_id = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&plans).Error

	if err != nil {
		return nil, 0, err
	}

	return plans, total, nil
}

// Update saves changes to a plan.
func (r *PlanRepository) Update(plan *SpendingPlan) error {
	return r.db.Save(plan).Error
}

// UpdateWithTx saves changes to a plan inside a transaction.
func (r *PlanRepository) UpdateWithTx(tx *gorm.DB, plan *SpendingPlan) error {
	return tx.Save(plan).Error
}

// UpdateDraftPlan updates only editable fields while plan is still draft.
func (r *PlanRepository) UpdateDraftPlan(planID string, userID string, updates map[string]interface{}) (*SpendingPlan, error) {
	var plan SpendingPlan

	err := r.db.Transaction(func(tx *gorm.DB) error {
		err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", planID, userID).
			First(&plan).Error

		if err != nil {
			return err
		}

		if plan.Status != PlanStatusDraft {
			return ErrPlanNotDraft
		}

		if len(updates) == 0 {
			return nil
		}

		err = tx.
			Model(&plan).
			Updates(updates).Error

		if err != nil {
			return err
		}

		return tx.
			Where("id = ? AND user_id = ?", planID, userID).
			First(&plan).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &plan, nil
}

// MarkAsActive confirms a plan.
// Wallet balance locking + wallet ledger creation should happen in wallet service within the same transaction later.
func (r *PlanRepository) MarkAsActive(tx *gorm.DB, plan *SpendingPlan) error {
	now := time.Now()

	plan.Status = PlanStatusActive
	plan.ConfirmedAt = &now

	return tx.Save(plan).Error
}

// MarkAsCancelled cancels a plan.
func (r *PlanRepository) MarkAsCancelled(tx *gorm.DB, plan *SpendingPlan, reason string) error {
	now := time.Now()

	plan.Status = PlanStatusCancelled
	plan.CancelledAt = &now
	plan.CancelReason = reason

	return tx.Save(plan).Error
}

// MarkAsCompleted marks a plan as completed.
func (r *PlanRepository) MarkAsCompleted(tx *gorm.DB, plan *SpendingPlan) error {
	now := time.Now()

	plan.Status = PlanStatusCompleted
	plan.CompletedAt = &now

	return tx.Save(plan).Error
}

// MarkAsExpired marks a plan as expired.
func (r *PlanRepository) MarkAsExpired(tx *gorm.DB, plan *SpendingPlan) error {
	now := time.Now()

	plan.Status = PlanStatusExpired
	plan.ExpiredAt = &now

	return tx.Save(plan).Error
}

// DeleteDraft soft-deletes a draft plan.
func (r *PlanRepository) DeleteDraft(planID string, userID string) error {
	result := r.db.
		Where("id = ? AND user_id = ? AND status = ?", planID, userID, PlanStatusDraft).
		Delete(&SpendingPlan{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrPlanNotFound
	}

	return nil
}

// ExistsForUser checks if a plan exists for a user.
func (r *PlanRepository) ExistsForUser(planID string, userID string) (bool, error) {
	var count int64

	err := r.db.
		Model(&SpendingPlan{}).
		Where("id = ? AND user_id = ?", planID, userID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CountByUserID returns total plans owned by a user.
func (r *PlanRepository) CountByUserID(userID string) (int64, error) {
	var count int64

	err := r.db.
		Model(&SpendingPlan{}).
		Where("user_id = ?", userID).
		Count(&count).Error

	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountActiveByUserID returns active plan count for dashboard/summary.
func (r *PlanRepository) CountActiveByUserID(userID string) (int64, error) {
	var count int64

	err := r.db.
		Model(&SpendingPlan{}).
		Where("user_id = ? AND status = ?", userID, PlanStatusActive).
		Count(&count).Error

	if err != nil {
		return 0, err
	}

	return count, nil
}

// SumActivePlanAmountByUserID returns total active locked plan amount.
// This can help wallet summary later.
func (r *PlanRepository) SumActivePlanAmountByUserID(userID string) (int64, error) {
	var total int64

	err := r.db.
		Model(&SpendingPlan{}).
		Where("user_id = ? AND status = ?", userID, PlanStatusActive).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, err
	}

	return total, nil
}

// WithTransaction exposes a transaction wrapper for service layer orchestration.
// Use this when plan confirmation needs to update plan + wallet + ledger atomically.
func (r *PlanRepository) WithTransaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// DB returns the underlying db.
// Use sparingly. Prefer repository methods.
func (r *PlanRepository) DB() *gorm.DB {
	return r.db
}
