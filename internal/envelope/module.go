package envelope

import (
	"github.com/kuipid01/salary-vault/internal/funds"
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"gorm.io/gorm"
)

type EnvelopeModule struct {
	Repo *EnvelopeRepo
	Svc  *EnvelopeService
	Hnd  *EnvelopeHandler
}

func NewEnvelopeModule(db *gorm.DB, planRepo *spendingplans.PlanRepository, subRepo *subscription.SubscriptionRepository, fundsRepo *funds.Repository) *EnvelopeModule {
	repo := NewEnvelopeRepo(db)
	svc := NewEnvelopeService(repo, planRepo, subRepo, fundsRepo)
	handler := NewEnvelopeHandler(svc)

	return &EnvelopeModule{
		Repo: repo,
		Svc:  svc,
		Hnd:  handler,
	}
}
