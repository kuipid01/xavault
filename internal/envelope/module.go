package envelope

import (
	spendingplans "github.com/kuipid01/salary-vault/internal/spending_plans"
	"gorm.io/gorm"
)

type EnvelopeModule struct {
	Repo *EnvelopeRepo
	Svc  *EnvelopeService
	Hnd  *EnvelopeHandler
}

func NewEnvelopeModule(db *gorm.DB, planRepo *spendingplans.PlanRepository) *EnvelopeModule {
	repo := NewRepo(db)
	svc := NewEnvelopeService(repo, planRepo)
	handler := NewEnvelopeHandler(svc)

	return &EnvelopeModule{
		Repo: repo,
		Svc:  svc,
		Hnd:  handler,
	}
}
