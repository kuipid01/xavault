-- +goose Up
CREATE TABLE provider_fee_rules (
    id uuid PRIMARY KEY,
    provider varchar(40) NOT NULL,
    operation varchar(50) NOT NULL,
    channel varchar(50) NOT NULL,
    currency varchar(3) NOT NULL DEFAULT 'NGN',
    percentage_basis_points bigint NOT NULL DEFAULT 0,
    fixed_fee_kobo bigint NOT NULL DEFAULT 0,
    fee_waiver_threshold_kobo bigint,
    maximum_fee_kobo bigint,
    minimum_amount_kobo bigint NOT NULL DEFAULT 1,
    maximum_amount_kobo bigint,
    government_levy_kobo bigint NOT NULL DEFAULT 0,
    government_levy_threshold_kobo bigint,
    version varchar(80) NOT NULL,
    effective_from timestamptz NOT NULL,
    effective_until timestamptz,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_provider_fee_rules_version_operation_channel
    ON provider_fee_rules(version, operation, channel);
CREATE INDEX idx_provider_fee_rules_lookup
    ON provider_fee_rules(provider, operation, channel, currency, enabled, effective_from);

INSERT INTO provider_fee_rules (
    id, provider, operation, channel, percentage_basis_points, fixed_fee_kobo,
    fee_waiver_threshold_kobo, maximum_fee_kobo, government_levy_kobo,
    government_levy_threshold_kobo, version, effective_from
) VALUES
    (gen_random_uuid(), 'paystack', 'deposit', 'checkout', 150, 10000, 250000, 200000, 0, NULL, 'paystack-ng-2026-01', NOW()),
    (gen_random_uuid(), 'paystack', 'deposit', 'dedicated_account', 100, 0, NULL, 30000, 0, NULL, 'paystack-ng-2026-01', NOW()),
    (gen_random_uuid(), 'paystack', 'withdrawal', 'transfer_0_5000', 0, 1000, NULL, NULL, 0, NULL, 'paystack-ng-2026-01', NOW()),
    (gen_random_uuid(), 'paystack', 'withdrawal', 'transfer_5001_50000', 0, 2500, NULL, NULL, 5000, 1000000, 'paystack-ng-2026-01', NOW()),
    (gen_random_uuid(), 'paystack', 'withdrawal', 'transfer_50001_plus', 0, 5000, NULL, NULL, 5000, 1000000, 'paystack-ng-2026-01', NOW());

CREATE TABLE fee_quotes (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    operation varchar(50) NOT NULL,
    provider varchar(40) NOT NULL,
    channel varchar(50) NOT NULL,
    currency varchar(3) NOT NULL DEFAULT 'NGN',
    principal_amount bigint NOT NULL CHECK (principal_amount > 0),
    provider_fee bigint NOT NULL DEFAULT 0,
    government_levy bigint NOT NULL DEFAULT 0,
    xavault_subsidy bigint NOT NULL DEFAULT 0,
    user_fee bigint NOT NULL DEFAULT 0,
    gross_charge bigint NOT NULL,
    net_credit bigint NOT NULL,
    total_deduction bigint NOT NULL,
    recipient_amount bigint NOT NULL,
    allowance_applied boolean NOT NULL DEFAULT false,
    pricing_version varchar(80) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'active',
    context_json jsonb,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fee_quotes_user_status ON fee_quotes(user_id, status, expires_at);

ALTER TABLE subscription_plans ADD COLUMN free_withdrawals_monthly bigint NOT NULL DEFAULT 0;
UPDATE subscription_plans SET free_withdrawals_monthly = CASE code
    WHEN 'free' THEN 0 WHEN 'lite' THEN 1 WHEN 'premium' THEN 3 WHEN 'power' THEN 8 ELSE 0 END;
ALTER TABLE monthly_usages ADD COLUMN free_withdrawals_used bigint NOT NULL DEFAULT 0;
ALTER TABLE monthly_usages ADD COLUMN free_withdrawals_reserved bigint NOT NULL DEFAULT 0;

ALTER TABLE wallet_deposits ADD COLUMN quote_id uuid REFERENCES fee_quotes(id);
ALTER TABLE wallet_deposits ADD COLUMN gross_amount bigint;
ALTER TABLE wallet_deposits ADD COLUMN wallet_credit_amount bigint;
ALTER TABLE wallet_deposits ADD COLUMN provider_fee bigint NOT NULL DEFAULT 0;
ALTER TABLE wallet_deposits ADD COLUMN pricing_version varchar(80);
UPDATE wallet_deposits SET gross_amount = amount, wallet_credit_amount = amount WHERE gross_amount IS NULL;

ALTER TABLE withdrawals ADD COLUMN quote_id uuid REFERENCES fee_quotes(id);
ALTER TABLE withdrawals ADD COLUMN provider_transfer_fee bigint NOT NULL DEFAULT 0;
ALTER TABLE withdrawals ADD COLUMN government_levy bigint NOT NULL DEFAULT 0;
ALTER TABLE withdrawals ADD COLUMN xavault_subsidy bigint NOT NULL DEFAULT 0;
ALTER TABLE withdrawals ADD COLUMN user_fee bigint NOT NULL DEFAULT 0;
ALTER TABLE withdrawals ADD COLUMN total_deduction bigint;
ALTER TABLE withdrawals ADD COLUMN recipient_amount bigint;
ALTER TABLE withdrawals ADD COLUMN allowance_applied boolean NOT NULL DEFAULT false;
ALTER TABLE withdrawals ADD COLUMN allowance_state varchar(20) NOT NULL DEFAULT 'none';
ALTER TABLE withdrawals ADD COLUMN allowance_period varchar(7);
ALTER TABLE withdrawals ADD COLUMN pricing_version varchar(80);
UPDATE withdrawals SET total_deduction = amount, recipient_amount = amount WHERE total_deduction IS NULL;

-- +goose Down
ALTER TABLE withdrawals DROP COLUMN IF EXISTS pricing_version, DROP COLUMN IF EXISTS allowance_period, DROP COLUMN IF EXISTS allowance_state,
    DROP COLUMN IF EXISTS allowance_applied, DROP COLUMN IF EXISTS recipient_amount,
    DROP COLUMN IF EXISTS total_deduction, DROP COLUMN IF EXISTS user_fee,
    DROP COLUMN IF EXISTS xavault_subsidy, DROP COLUMN IF EXISTS government_levy,
    DROP COLUMN IF EXISTS provider_transfer_fee, DROP COLUMN IF EXISTS quote_id;
ALTER TABLE wallet_deposits DROP COLUMN IF EXISTS pricing_version, DROP COLUMN IF EXISTS provider_fee,
    DROP COLUMN IF EXISTS wallet_credit_amount, DROP COLUMN IF EXISTS gross_amount, DROP COLUMN IF EXISTS quote_id;
ALTER TABLE monthly_usages DROP COLUMN IF EXISTS free_withdrawals_reserved, DROP COLUMN IF EXISTS free_withdrawals_used;
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS free_withdrawals_monthly;
DROP TABLE IF EXISTS fee_quotes;
DROP TABLE IF EXISTS provider_fee_rules;
