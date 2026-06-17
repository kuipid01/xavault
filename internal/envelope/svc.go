package envelope

import (
	"strings"
	"time"

	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/shared/security"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"gorm.io/gorm"
)

type EnvelopeService struct {
	repo     *EnvelopeRepo
	planRepo *spendingplans.PlanRepository
}

func NewEnvelopeService(repo *EnvelopeRepo, planRepo *spendingplans.PlanRepository) *EnvelopeService {
	return &EnvelopeService{
		repo:     repo,
		planRepo: planRepo,
	}
}

func (s *EnvelopeService) CreateEnvelope(userID string, planID string, req CreateEnvelopeRequest) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	validationErrors := validateCreateEnvelopeRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	startDate, err := parseEnvelopeDate(req.StartDate)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid start date",
			Error: map[string]string{
				"start_date": "Start date must be in YYYY-MM-DD format",
			},
		}
	}

	endDate, err := parseEnvelopeDate(req.EndDate)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid end date",
			Error: map[string]string{
				"end_date": "End date must be in YYYY-MM-DD format",
			},
		}
	}

	if !endDate.After(startDate) {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid date range",
			Error: map[string]string{
				"end_date": "End date must be after start date",
			},
		}
	}

	var createdEnvelope *Envelope

	err = s.repo.WithTransaction(func(tx *gorm.DB) error {
		plan, err := s.planRepo.FindByIDAndUserIDForUpdate(tx, planID, userID)
		if err != nil {
			return err
		}

		if plan == nil {
			return ErrPlanNotFound
		}

		if plan.Status != spendingplans.PlanStatusDraft {
			return ErrPlanNotDraft
		}

		parentEnvelopeID := normalizeOptionalString(req.ParentEnvelopeID)

		if parentEnvelopeID == nil {
			if startDate.Before(plan.StartDate) || endDate.After(plan.EndDate) {
				return ErrEnvelopeInvalidDateRange
			}

			currentTopLevelTotal, err := s.repo.SumTopLevelEnvelopeAmountTx(tx, planID)
			if err != nil {
				return err
			}

			if currentTopLevelTotal+req.Amount > plan.Amount {
				return ErrEnvelopeAmountExceeded
			}
		} else {
			parent, err := s.repo.FindByIDPlanIDAndUserIDForUpdate(tx, *parentEnvelopeID, planID, userID)
			if err != nil {
				return err
			}

			if parent == nil {
				return ErrEnvelopeParentNotFound
			}

			if startDate.Before(parent.StartDate) || endDate.After(parent.EndDate) {
				return ErrEnvelopeInvalidDateRange
			}

			currentChildTotal, err := s.repo.SumChildEnvelopeAmountTx(tx, parent.ID)
			if err != nil {
				return err
			}

			if currentChildTotal+req.Amount > parent.Amount {
				return ErrEnvelopeAmountExceeded
			}
		}

		policy := strings.TrimSpace(req.UnboxedBalancePolicy)
		if policy == "" {
			policy = EnvelopeUnboxedPolicyFlexible
		}

		envelope := &Envelope{
			ID:                   security.GenateUniqueID(),
			UserID:               userID,
			PlanID:               planID,
			ParentEnvelopeID:     parentEnvelopeID,
			Name:                 strings.TrimSpace(req.Name),
			Amount:               req.Amount,
			Status:               EnvelopeStatusActive,
			UnboxedBalancePolicy: policy,
			StartDate:            startDate,
			EndDate:              endDate,
		}

		err = s.repo.CreateWithTx(tx, envelope)
		if err != nil {
			return err
		}

		createdEnvelope = envelope
		return nil
	})

	if err != nil {
		return envelopeErrorResponse("Could not create envelope", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Envelope created successfully",
		Data:    mapEnvelopeToResponse(createdEnvelope, 0, nil),
	}
}

func (s *EnvelopeService) GetPlanEnvelopes(userID string, planID string) *response.APIResponse {
	planID = strings.TrimSpace(planID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	plan, err := s.planRepo.FindByIDAndUserID(planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch spending plan",
			Error:   err,
		}
	}

	if plan == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan not found",
			Error:   ErrPlanNotFound,
		}
	}

	envelopes, err := s.repo.FindByPlanIDAndUserID(planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch envelopes",
			Error:   err,
		}
	}

	tree := buildEnvelopeTree(envelopes)

	return &response.APIResponse{
		Success: true,
		Message: "Envelopes fetched successfully",
		Data:    tree,
	}
}

func (s *EnvelopeService) GetEnvelopeByID(userID string, planID string, envelopeID string) *response.APIResponse {
	planID = strings.TrimSpace(planID)
	envelopeID = strings.TrimSpace(envelopeID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	if envelopeID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		}
	}

	envelope, err := s.repo.FindByIDPlanIDAndUserID(envelopeID, planID, userID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch envelope",
			Error:   err,
		}
	}

	if envelope == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope not found",
			Error:   ErrEnvelopeNotFound,
		}
	}

	allocatedToChildren, err := s.repo.SumChildEnvelopeAmount(envelope.ID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not calculate envelope balance",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Envelope fetched successfully",
		Data:    mapEnvelopeToResponse(envelope, allocatedToChildren, nil),
	}
}

func (s *EnvelopeService) UpdateEnvelope(userID string, planID string, envelopeID string, req UpdateEnvelopeRequest) *response.APIResponse {
	planID = strings.TrimSpace(planID)
	envelopeID = strings.TrimSpace(envelopeID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	if envelopeID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		}
	}

	validationErrors := validateUpdateEnvelopeRequest(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	var updatedEnvelope *Envelope

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		plan, err := s.planRepo.FindByIDAndUserIDForUpdate(tx, planID, userID)
		if err != nil {
			return err
		}

		if plan == nil {
			return ErrPlanNotFound
		}

		if plan.Status != spendingplans.PlanStatusDraft {
			return ErrPlanNotDraft
		}

		envelope, err := s.repo.FindByIDPlanIDAndUserIDForUpdate(tx, envelopeID, planID, userID)
		if err != nil {
			return err
		}

		if envelope == nil {
			return ErrEnvelopeNotFound
		}

		newName := envelope.Name
		newAmount := envelope.Amount
		newStartDate := envelope.StartDate
		newEndDate := envelope.EndDate
		newPolicy := envelope.UnboxedBalancePolicy

		if req.Name != nil {
			newName = strings.TrimSpace(*req.Name)
		}

		if req.Amount != nil {
			newAmount = *req.Amount
		}

		if req.StartDate != nil {
			parsedStartDate, err := parseEnvelopeDate(*req.StartDate)
			if err != nil {
				return ErrEnvelopeInvalidDateRange
			}

			newStartDate = parsedStartDate
		}

		if req.EndDate != nil {
			parsedEndDate, err := parseEnvelopeDate(*req.EndDate)
			if err != nil {
				return ErrEnvelopeInvalidDateRange
			}

			newEndDate = parsedEndDate
		}

		if req.UnboxedBalancePolicy != nil {
			newPolicy = strings.TrimSpace(*req.UnboxedBalancePolicy)
		}

		if !newEndDate.After(newStartDate) {
			return ErrEnvelopeInvalidDateRange
		}

		childTotal, err := s.repo.SumChildEnvelopeAmountTx(tx, envelope.ID)
		if err != nil {
			return err
		}

		if newAmount < childTotal {
			return ErrEnvelopeAmountExceeded
		}

		if envelope.ParentEnvelopeID == nil {
			if newStartDate.Before(plan.StartDate) || newEndDate.After(plan.EndDate) {
				return ErrEnvelopeInvalidDateRange
			}

			topLevelTotalExcludingCurrent, err := s.repo.SumTopLevelEnvelopeAmountExcludingTx(tx, planID, envelope.ID)
			if err != nil {
				return err
			}

			if topLevelTotalExcludingCurrent+newAmount > plan.Amount {
				return ErrEnvelopeAmountExceeded
			}
		} else {
			parent, err := s.repo.FindByIDPlanIDAndUserIDForUpdate(tx, *envelope.ParentEnvelopeID, planID, userID)
			if err != nil {
				return err
			}

			if parent == nil {
				return ErrEnvelopeParentNotFound
			}

			if newStartDate.Before(parent.StartDate) || newEndDate.After(parent.EndDate) {
				return ErrEnvelopeInvalidDateRange
			}

			siblingTotalExcludingCurrent, err := s.repo.SumChildEnvelopeAmountExcludingTx(tx, parent.ID, envelope.ID)
			if err != nil {
				return err
			}

			if siblingTotalExcludingCurrent+newAmount > parent.Amount {
				return ErrEnvelopeAmountExceeded
			}
		}

		envelope.Name = newName
		envelope.Amount = newAmount
		envelope.StartDate = newStartDate
		envelope.EndDate = newEndDate
		envelope.UnboxedBalancePolicy = newPolicy

		err = s.repo.UpdateWithTx(tx, envelope)
		if err != nil {
			return err
		}

		updatedEnvelope = envelope
		return nil
	})

	if err != nil {
		return envelopeErrorResponse("Could not update envelope", err)
	}

	allocatedToChildren, err := s.repo.SumChildEnvelopeAmount(updatedEnvelope.ID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope updated, but could not calculate balance",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Envelope updated successfully",
		Data:    mapEnvelopeToResponse(updatedEnvelope, allocatedToChildren, nil),
	}
}

func (s *EnvelopeService) DeleteEnvelope(userID string, planID string, envelopeID string) *response.APIResponse {
	planID = strings.TrimSpace(planID)
	envelopeID = strings.TrimSpace(envelopeID)

	if planID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Plan ID is required",
			Error:   "plan_id_required",
		}
	}

	if envelopeID == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope ID is required",
			Error:   "envelope_id_required",
		}
	}

	err := s.repo.WithTransaction(func(tx *gorm.DB) error {
		plan, err := s.planRepo.FindByIDAndUserIDForUpdate(tx, planID, userID)
		if err != nil {
			return err
		}

		if plan == nil {
			return ErrPlanNotFound
		}

		if plan.Status != spendingplans.PlanStatusDraft {
			return ErrPlanNotDraft
		}

		envelope, err := s.repo.FindByIDPlanIDAndUserIDForUpdate(tx, envelopeID, planID, userID)
		if err != nil {
			return err
		}

		if envelope == nil {
			return ErrEnvelopeNotFound
		}

		childCount, err := s.repo.CountChildrenTx(tx, envelope.ID)
		if err != nil {
			return err
		}

		if childCount > 0 {
			return ErrEnvelopeHasChildren
		}

		return s.repo.DeleteWithTx(tx, envelope)
	})

	if err != nil {
		return envelopeErrorResponse("Could not delete envelope", err)
	}

	return &response.APIResponse{
		Success: true,
		Message: "Envelope deleted successfully",
		Data: map[string]string{
			"envelope_id": envelopeID,
		},
	}
}

func parseEnvelopeDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func validateCreateEnvelopeRequest(req CreateEnvelopeRequest) map[string]string {
	errors := map[string]string{}

	if strings.TrimSpace(req.Name) == "" {
		errors["name"] = "Name is required"
	}

	if len(strings.TrimSpace(req.Name)) > 120 {
		errors["name"] = "Name cannot be more than 120 characters"
	}

	if req.Amount <= 0 {
		errors["amount"] = "Amount must be greater than zero"
	}

	if strings.TrimSpace(req.StartDate) == "" {
		errors["start_date"] = "Start date is required"
	}

	if strings.TrimSpace(req.EndDate) == "" {
		errors["end_date"] = "End date is required"
	}

	policy := strings.TrimSpace(req.UnboxedBalancePolicy)
	if policy != "" && !isValidEnvelopeUnboxedPolicy(policy) {
		errors["unboxed_balance_policy"] = "Invalid unboxed balance policy"
	}

	return errors
}

func validateUpdateEnvelopeRequest(req UpdateEnvelopeRequest) map[string]string {
	errors := map[string]string{}

	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		errors["name"] = "Name cannot be empty"
	}

	if req.Name != nil && len(strings.TrimSpace(*req.Name)) > 120 {
		errors["name"] = "Name cannot be more than 120 characters"
	}

	if req.Amount != nil && *req.Amount <= 0 {
		errors["amount"] = "Amount must be greater than zero"
	}

	if req.UnboxedBalancePolicy != nil && !isValidEnvelopeUnboxedPolicy(*req.UnboxedBalancePolicy) {
		errors["unboxed_balance_policy"] = "Invalid unboxed balance policy"
	}

	return errors
}

func isValidEnvelopeUnboxedPolicy(policy string) bool {
	switch strings.TrimSpace(policy) {
	case EnvelopeUnboxedPolicyFlexible, EnvelopeUnboxedPolicyLockedUntilParentEnds:
		return true
	default:
		return false
	}
}

func envelopeErrorResponse(message string, err error) *response.APIResponse {
	if err == ErrPlanNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Spending plan not found",
			Error:   err,
		}
	}

	if err == ErrPlanNotDraft {
		return &response.APIResponse{
			Success: false,
			Message: "Only draft plans can be edited",
			Error:   err,
		}
	}

	if err == ErrEnvelopeNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope not found",
			Error:   err,
		}
	}

	if err == ErrEnvelopeParentNotFound {
		return &response.APIResponse{
			Success: false,
			Message: "Parent envelope not found",
			Error:   err,
		}
	}

	if err == ErrEnvelopeAmountExceeded {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope amount exceeds available allocation",
			Error:   err,
		}
	}

	if err == ErrEnvelopeInvalidDateRange {
		return &response.APIResponse{
			Success: false,
			Message: "Envelope date must be within its parent date range",
			Error:   err,
		}
	}

	if err == ErrEnvelopeHasChildren {
		return &response.APIResponse{
			Success: false,
			Message: "Cannot delete an envelope that has child envelopes",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: false,
		Message: message,
		Error:   err,
	}
}
