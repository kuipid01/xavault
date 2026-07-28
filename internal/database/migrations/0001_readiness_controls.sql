CREATE TABLE IF NOT EXISTS paystack_events (
    id uuid PRIMARY KEY,
    provider_event_id varchar(220) NOT NULL UNIQUE,
    event_type varchar(80) NOT NULL,
    resource_id bigint NOT NULL DEFAULT 0,
    resource_reference varchar(140),
    payload_hash char(64) NOT NULL,
    payload text NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    next_retry_at timestamptz,
    processing_started_at timestamptz,
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_paystack_events_status ON paystack_events(status);
CREATE INDEX IF NOT EXISTS idx_paystack_events_next_retry_at ON paystack_events(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_paystack_events_resource_reference ON paystack_events(resource_reference);

CREATE TABLE IF NOT EXISTS reconciliation_controls (
    id bigint PRIMARY KEY,
    processing_halted boolean NOT NULL DEFAULT false,
    reason text,
    halted_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
INSERT INTO reconciliation_controls(id, processing_halted, updated_at)
VALUES (1, false, NOW()) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS reconciliation_findings (
    id bigserial PRIMARY KEY,
    run_id uuid NOT NULL,
    severity varchar(20) NOT NULL,
    code varchar(80) NOT NULL,
    entity_type varchar(40) NOT NULL,
    entity_id varchar(120),
    expected text,
    actual text,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reconciliation_findings_run_id ON reconciliation_findings(run_id);
CREATE INDEX IF NOT EXISTS idx_reconciliation_findings_unresolved ON reconciliation_findings(resolved_at);

CREATE TABLE IF NOT EXISTS reconciliation_operator_actions (
    id bigserial PRIMARY KEY,
    operator_id varchar(120) NOT NULL,
    action varchar(80) NOT NULL,
    reference varchar(140),
    details text,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reconciliation_operator_actions_reference
ON reconciliation_operator_actions(reference);
