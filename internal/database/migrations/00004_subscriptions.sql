-- +goose Up
-- Baseline domain migration. Idempotent for adoption by legacy databases.

CREATE TABLE IF NOT EXISTS subscription_plans (
    id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    code text NOT NULL,
    name text NOT NULL,
    price_kobo bigint NOT NULL CHECK (price_kobo >= 0),
    currency varchar(3) NOT NULL DEFAULT 'NGN',
    max_monthly_volume_kobo bigint NOT NULL,
    max_locked_plans bigint NOT NULL,
    max_flexible_envelopes bigint NOT NULL,
    max_emergency_vaults bigint NOT NULL,
    free_flexible_withdrawals_monthly bigint NOT NULL,
    free_emergency_withdrawals_monthly bigint NOT NULL,
    has_salary_split boolean NOT NULL DEFAULT false,
    has_full_analytics boolean NOT NULL DEFAULT false,
    has_shared_envelopes boolean NOT NULL DEFAULT false,
    has_advanced_rules boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_plans_code ON subscription_plans(code);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_deleted_at ON subscription_plans(deleted_at);

CREATE TABLE IF NOT EXISTS user_subscriptions (
    id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    user_id uuid NOT NULL REFERENCES users(id),
    plan_id uuid NOT NULL REFERENCES subscription_plans(id),
    status text NOT NULL,
    current_period_start timestamptz NOT NULL,
    current_period_end timestamptz NOT NULL,
    provider varchar(50),
    provider_ref text,
    payment_reference text,
    cancel_at_period_end boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan_id ON user_subscriptions(plan_id);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_status ON user_subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_current_period_end ON user_subscriptions(current_period_end);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_provider_ref ON user_subscriptions(provider_ref);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_payment_reference ON user_subscriptions(payment_reference);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_deleted_at ON user_subscriptions(deleted_at);

CREATE TABLE IF NOT EXISTS monthly_usages (
    id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    user_id uuid NOT NULL REFERENCES users(id),
    period_key varchar(7) NOT NULL,
    managed_volume_kobo bigint NOT NULL DEFAULT 0,
    locked_plans_created bigint NOT NULL DEFAULT 0,
    flexible_envelopes_created bigint NOT NULL DEFAULT 0,
    emergency_vaults_created bigint NOT NULL DEFAULT 0,
    free_flexible_withdrawals_used bigint NOT NULL DEFAULT 0,
    free_emergency_withdrawals_used bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_monthly_usage_user_period ON monthly_usages(user_id, period_key);
CREATE INDEX IF NOT EXISTS idx_monthly_usages_deleted_at ON monthly_usages(deleted_at);

