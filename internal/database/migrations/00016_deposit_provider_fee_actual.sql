-- +goose Up
-- Capture the fee the provider actually charged, next to the rule-based
-- estimate (provider_fee), so fee accuracy can be audited per transaction.
ALTER TABLE wallet_deposits ADD COLUMN IF NOT EXISTS provider_fee_actual bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE wallet_deposits DROP COLUMN IF EXISTS provider_fee_actual;
