-- +goose Up
-- Lease columns are separate so legacy databases can adopt worker leasing.
ALTER TABLE paystack_events
ADD COLUMN IF NOT EXISTS lease_expires_at timestamptz;
ALTER TABLE paystack_events
ADD COLUMN IF NOT EXISTS lease_token uuid;

CREATE INDEX IF NOT EXISTS idx_paystack_events_lease_expires_at
ON paystack_events(lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_paystack_events_lease_token
ON paystack_events(lease_token);
