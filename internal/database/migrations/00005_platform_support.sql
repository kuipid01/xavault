-- +goose Up
-- Baseline domain migration. Idempotent for adoption by legacy databases.

CREATE TABLE IF NOT EXISTS idempotent_keys (
    key text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    state text NOT NULL,
    response_code bigint,
    response_body text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    locked_until timestamptz
);
CREATE INDEX IF NOT EXISTS idx_idempotent_keys_user_id ON idempotent_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_idempotent_keys_created_at ON idempotent_keys(created_at);
ALTER TABLE idempotent_keys ADD COLUMN IF NOT EXISTS request_hash text;

CREATE TABLE IF NOT EXISTS entries (
    id uuid PRIMARY KEY,
    email varchar(320) NOT NULL,
    source varchar(80) NOT NULL DEFAULT 'landing_page',
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_email ON entries(lower(email));

