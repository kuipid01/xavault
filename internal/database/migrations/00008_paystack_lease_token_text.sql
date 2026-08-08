-- +goose Up
-- Empty means "not leased" in the event state machine. Store it as text so
-- initial inserts and lease release do not violate PostgreSQL UUID parsing.
ALTER TABLE paystack_events
    ALTER COLUMN lease_token TYPE varchar(64)
    USING lease_token::text;

-- +goose Down
UPDATE paystack_events SET lease_token = NULL WHERE lease_token = '';
ALTER TABLE paystack_events
    ALTER COLUMN lease_token TYPE uuid
    USING lease_token::uuid;
