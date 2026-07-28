package envelope

import (
	"time"

	"github.com/kuipid01/salary-vault/internal/funds"
)

func mapEnvelopeToResponse(
	envelope *Envelope,
	allocatedToChildren int64,
	children []EnvelopeResponse,
	totals funds.WithdrawalTotals,
) EnvelopeResponse {
	if envelope == nil {
		return EnvelopeResponse{}
	}

	return EnvelopeResponse{
		ID:                      envelope.ID,
		UserID:                  envelope.UserID,
		PlanID:                  envelope.PlanID,
		ParentEnvelopeID:        envelope.ParentEnvelopeID,
		Name:                    envelope.Name,
		Amount:                  envelope.Amount,
		Status:                  envelope.Status,
		UnboxedBalancePolicy:    envelope.UnboxedBalancePolicy,
		StartDate:               formatEnvelopeDate(envelope.StartDate),
		EndDate:                 formatEnvelopeDate(envelope.EndDate),
		AllocatedToChildren:     allocatedToChildren,
		UnboxedBalance:          funds.AvailableBalance(envelope.Amount, allocatedToChildren, totals),
		WithdrawnAmount:         totals.Paid,
		PendingWithdrawalAmount: totals.Pending,
		Children:                children,
		CreatedAt:               formatEnvelopeTime(envelope.CreatedAt),
		UpdatedAt:               formatEnvelopeTime(envelope.UpdatedAt),
	}
}

func buildEnvelopeTree(envelopes []Envelope, totalsByEnvelope map[string]funds.WithdrawalTotals) []EnvelopeResponse {
	childrenByParent := map[string][]Envelope{}
	topLevel := []Envelope{}

	for _, env := range envelopes {
		if env.ParentEnvelopeID == nil {
			topLevel = append(topLevel, env)
			continue
		}

		childrenByParent[*env.ParentEnvelopeID] = append(childrenByParent[*env.ParentEnvelopeID], env)
	}

	var buildNode func(env Envelope) EnvelopeResponse

	buildNode = func(env Envelope) EnvelopeResponse {
		children := childrenByParent[env.ID]
		childResponses := make([]EnvelopeResponse, 0, len(children))

		var allocatedToChildren int64

		for _, child := range children {
			allocatedToChildren += child.Amount
			childResponses = append(childResponses, buildNode(child))
		}

		return mapEnvelopeToResponse(&env, allocatedToChildren, childResponses, totalsByEnvelope[env.ID])
	}

	responses := make([]EnvelopeResponse, 0, len(topLevel))

	for _, env := range topLevel {
		responses = append(responses, buildNode(env))
	}

	return responses
}

func formatEnvelopeDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format("2006-01-02")
}

func formatEnvelopeTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format(time.RFC3339)
}
