-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS gender text;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS gender;
