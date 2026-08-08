-- +goose Up
-- The Free plan is a baseline entitlement, not a payment product. Keep the
-- canonical row available even before application seeding runs.
INSERT INTO subscription_plans (
    id, code, name, price_kobo, currency, max_monthly_volume_kobo,
    max_locked_plans, max_flexible_envelopes, max_emergency_vaults,
    free_flexible_withdrawals_monthly, free_emergency_withdrawals_monthly,
    has_salary_split, has_full_analytics, has_shared_envelopes,
    has_advanced_rules, created_at, updated_at
)
VALUES (
    '00000000-0000-0000-0000-000000000001', 'free', 'Free', 0, 'NGN', 5000000,
    2, 2, 1, 0, 0, false, false, false, false, NOW(), NOW()
)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    price_kobo = EXCLUDED.price_kobo,
    currency = EXCLUDED.currency,
    max_monthly_volume_kobo = EXCLUDED.max_monthly_volume_kobo,
    max_locked_plans = EXCLUDED.max_locked_plans,
    max_flexible_envelopes = EXCLUDED.max_flexible_envelopes,
    max_emergency_vaults = EXCLUDED.max_emergency_vaults,
    updated_at = NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_subscriptions_active_user_plan
    ON user_subscriptions(user_id, plan_id)
    WHERE status = 'active' AND deleted_at IS NULL;

INSERT INTO user_subscriptions (
    id, user_id, plan_id, status, current_period_start, current_period_end,
    provider, cancel_at_period_end, created_at, updated_at
)
SELECT
    gen_random_uuid(), u.id, p.id, 'active', NOW(), NOW() + INTERVAL '100 years',
    'xavault', false, NOW(), NOW()
FROM users u
JOIN subscription_plans p ON p.code = 'free' AND p.deleted_at IS NULL
WHERE NOT EXISTS (
    SELECT 1 FROM user_subscriptions us
    WHERE us.user_id = u.id AND us.plan_id = p.id
      AND us.status = 'active' AND us.deleted_at IS NULL
);

-- +goose Down
DROP INDEX IF EXISTS idx_user_subscriptions_active_user_plan;
DELETE FROM user_subscriptions us
USING subscription_plans p
WHERE us.plan_id = p.id AND p.code = 'free' AND us.provider = 'xavault';
