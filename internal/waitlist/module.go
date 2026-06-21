package waitlist

import "gorm.io/gorm"

type Module struct {
	Repo *Repository
	Svc  *Service
	Hnd  *Handler
}

func NewModule(db *gorm.DB) *Module {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Module{Repo: repo, Svc: service, Hnd: NewHandler(service)}
}
