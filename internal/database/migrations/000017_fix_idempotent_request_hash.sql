-- +goose Up

ALTER TABLE idempotent_keys
ADD COLUMN IF NOT EXISTS request_hash text;

-- +goose Down

ALTER TABLE idempotent_keys
DROP COLUMN IF EXISTS request_hash;
