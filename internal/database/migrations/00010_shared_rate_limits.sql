-- +goose Up
CREATE TABLE IF NOT EXISTS authentication_rate_limits (
    key_hash char(64) PRIMARY KEY,
    policy varchar(80) NOT NULL,
    attempt_count integer NOT NULL CHECK (attempt_count >= 0),
    window_started_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auth_rate_limits_expires_at ON authentication_rate_limits(expires_at);

-- +goose Down
DROP TABLE IF EXISTS authentication_rate_limits;
