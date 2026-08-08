package middleware

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type RateLimitResult struct {
	Count   int
	ResetAt time.Time
}

type RateLimitStore interface {
	Increment(context.Context, string, string, time.Duration) (RateLimitResult, error)
}

type PostgresRateLimitStore struct{ db *gorm.DB }

func NewPostgresRateLimitStore(db *gorm.DB) *PostgresRateLimitStore {
	return &PostgresRateLimitStore{db: db}
}

func (s *PostgresRateLimitStore) Increment(ctx context.Context, keyHash, policy string, window time.Duration) (RateLimitResult, error) {
	var result RateLimitResult
	seconds := int64(window.Seconds())
	err := s.db.WithContext(ctx).Raw(`
INSERT INTO authentication_rate_limits
    (key_hash, policy, attempt_count, window_started_at, expires_at, updated_at)
VALUES (?, ?, 1, NOW(), NOW() + (? * INTERVAL '1 second'), NOW())
ON CONFLICT (key_hash) DO UPDATE SET
    policy = EXCLUDED.policy,
    attempt_count = CASE WHEN authentication_rate_limits.expires_at <= NOW() THEN 1 ELSE authentication_rate_limits.attempt_count + 1 END,
    window_started_at = CASE WHEN authentication_rate_limits.expires_at <= NOW() THEN NOW() ELSE authentication_rate_limits.window_started_at END,
    expires_at = CASE WHEN authentication_rate_limits.expires_at <= NOW() THEN NOW() + (? * INTERVAL '1 second') ELSE authentication_rate_limits.expires_at END,
    updated_at = NOW()
RETURNING attempt_count AS count, expires_at AS reset_at`, keyHash, policy, seconds, seconds).Scan(&result).Error
	return result, err
}

func (s *PostgresRateLimitStore) DeleteExpired(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("DELETE FROM authentication_rate_limits WHERE expires_at < NOW() - INTERVAL '1 day'").Error
}
