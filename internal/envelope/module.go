package envelope

import (
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"gorm.io/gorm"
)

type EnvelopeModule struct {
	Repo *EnvelopeRepo
	Svc  *EnvelopeService
	Hnd  *EnvelopeHandler
}

func NewEnvelopeModule(db *gorm.DB, planRepo *spendingplans.PlanRepository, subRepo *subscription.SubscriptionRepository) *EnvelopeModule {
	repo := NewEnvelopeRepo(db)
	svc := NewEnvelopeService(repo, planRepo, subRepo)
	handler := NewEnvelopeHandler(svc)

	return &EnvelopeModule{
		Repo: repo,
		Svc:  svc,
		Hnd:  handler,
	}
}
