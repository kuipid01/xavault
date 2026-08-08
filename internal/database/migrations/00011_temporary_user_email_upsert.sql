-- +goose Up
-- AuthRepository uses ON CONFLICT (email). PostgreSQL requires an exact-column
-- unique index for that conflict target; lower(email) alone is not eligible.
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporary_users_email_exact
    ON temporary_users(email);

-- +goose Down
DROP INDEX IF EXISTS idx_temporary_users_email_exact;
