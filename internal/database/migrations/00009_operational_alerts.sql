-- +goose Up
CREATE TABLE IF NOT EXISTS operational_alerts (
    id uuid PRIMARY KEY,
    severity varchar(20) NOT NULL,
    event_type varchar(100) NOT NULL,
    title varchar(200) NOT NULL,
    message text NOT NULL,
    fingerprint varchar(180) NOT NULL,
    dedupe_key varchar(240) NOT NULL,
    metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_operational_alerts_dedupe_key ON operational_alerts(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_operational_alerts_event_created ON operational_alerts(event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS operational_alert_deliveries (
    id uuid PRIMARY KEY,
    alert_id uuid NOT NULL REFERENCES operational_alerts(id) ON DELETE CASCADE,
    destination varchar(80) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz,
    last_error text,
    delivered_at timestamptz,
    lease_token varchar(64) NOT NULL DEFAULT '',
    lease_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE(alert_id, destination)
);
CREATE INDEX IF NOT EXISTS idx_alert_deliveries_due ON operational_alert_deliveries(status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_alert_deliveries_lease ON operational_alert_deliveries(lease_expires_at);

-- +goose Down
DROP TABLE IF EXISTS operational_alert_deliveries;
DROP TABLE IF EXISTS operational_alerts;
