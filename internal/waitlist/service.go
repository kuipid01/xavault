package waitlist

import (
	"errors"
	"net/mail"
	"strings"
)

var ErrInvalidEmail = errors.New("a valid email address is required")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Join(req JoinRequest) (*JoinResponse, bool, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 320 {
		return nil, false, ErrInvalidEmail
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "landing_page"
	}
	if len(source) > 80 {
		source = source[:80]
	}

	entry, created, err := s.repo.CreateOrFind(&Entry{Email: email, Source: source})
	if err != nil {
		return nil, false, err
	}

	return &JoinResponse{
		Email:         entry.Email,
		JoinedAt:      entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		AlreadyJoined: !created,
	}, created, nil
}
