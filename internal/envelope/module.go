package envelope

import (
	"gorm.io/gorm"
)

type EnvelopeModule struct {
	Repo *EnvelopeRepo
	Svc  *EnvelopeService
	Hnd  *EnvelopeHandler
}

func NewEnvelopeModule(db *gorm.DB) *EnvelopeModule {
	repo := NewRepo(db)
	svc := NewEnvelopeService(repo, nil)
	handler := NewEnvelopeHandler(svc)

	return &EnvelopeModule{
		Repo: repo,
		Svc:  svc,
		Hnd:  handler,
	}
}
