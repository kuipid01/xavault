-- +goose Up
-- Baseline domain migration. Idempotent for adoption by legacy databases.

CREATE TABLE IF NOT EXISTS spending_plans (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    name varchar(120) NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    currency varchar(10) NOT NULL DEFAULT 'NGN',
    status varchar(30) NOT NULL DEFAULT 'draft',
    funding_status varchar(30) NOT NULL DEFAULT 'unfunded',
    unboxed_balance_policy varchar(50) NOT NULL DEFAULT 'flexible',
    start_date timestamptz NOT NULL,
    end_date timestamptz NOT NULL,
    confirmed_at timestamptz,
    funded_at timestamptz,
    cancelled_at timestamptz,
    completed_at timestamptz,
    expired_at timestamptz,
    cancel_reason varchar(255),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    CHECK (end_date >= start_date)
);
CREATE INDEX IF NOT EXISTS idx_spending_plans_user_id ON spending_plans(user_id);
CREATE INDEX IF NOT EXISTS idx_spending_plans_wallet_id ON spending_plans(wallet_id);
CREATE INDEX IF NOT EXISTS idx_spending_plans_status ON spending_plans(status);
CREATE INDEX IF NOT EXISTS idx_spending_plans_funding_status ON spending_plans(funding_status);
CREATE INDEX IF NOT EXISTS idx_spending_plans_deleted_at ON spending_plans(deleted_at);

CREATE TABLE IF NOT EXISTS envelopes (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    plan_id uuid NOT NULL REFERENCES spending_plans(id),
    parent_envelope_id uuid REFERENCES envelopes(id),
    name varchar(120) NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    status varchar(30) NOT NULL DEFAULT 'active',
    unboxed_balance_policy varchar(50) NOT NULL DEFAULT 'flexible',
    start_date timestamptz NOT NULL,
    end_date timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    CHECK (end_date >= start_date)
);
CREATE INDEX IF NOT EXISTS idx_envelopes_user_id ON envelopes(user_id);
CREATE INDEX IF NOT EXISTS idx_envelopes_plan_id ON envelopes(plan_id);
CREATE INDEX IF NOT EXISTS idx_envelopes_parent_envelope_id ON envelopes(parent_envelope_id);
CREATE INDEX IF NOT EXISTS idx_envelopes_status ON envelopes(status);
CREATE INDEX IF NOT EXISTS idx_envelopes_deleted_at ON envelopes(deleted_at);

CREATE TABLE IF NOT EXISTS bank_accounts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    owner_type varchar(30) NOT NULL,
    nickname varchar(120),
    bank_name varchar(160) NOT NULL,
    bank_code varchar(30) NOT NULL,
    account_number varchar(20) NOT NULL,
    account_name varchar(180) NOT NULL,
    provider varchar(40) NOT NULL DEFAULT 'Paystack',
    recipient_code varchar(120) NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_user_id ON bank_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_owner_type ON bank_accounts(owner_type);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_bank_code ON bank_accounts(bank_code);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_account_number ON bank_accounts(account_number);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_recipient_code ON bank_accounts(recipient_code);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_deleted_at ON bank_accounts(deleted_at);

CREATE TABLE IF NOT EXISTS withdrawals (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    plan_id uuid NOT NULL REFERENCES spending_plans(id),
    envelope_id uuid REFERENCES envelopes(id),
    source_type varchar(40) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'pending',
    amount bigint NOT NULL CHECK (amount > 0),
    currency varchar(10) NOT NULL DEFAULT 'NGN',
    reason varchar(255),
    reference varchar(120) NOT NULL,
    description varchar(255),
    paid_at timestamptz,
    failed_at timestamptz,
    cancelled_at timestamptz,
    reversed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    requested_at timestamptz,
    transfer_initiated_at timestamptz,
    provider text,
    provider_status text,
    provider_reference text,
    provider_transfer_code text,
    provider_response_json text,
    recipient_code text,
    destination_type text,
    destination_id text,
    idempotency_key text,
    failure_reason text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_withdrawals_reference ON withdrawals(reference);
CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id ON withdrawals(user_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_wallet_id ON withdrawals(wallet_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_plan_id ON withdrawals(plan_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_envelope_id ON withdrawals(envelope_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_source_type ON withdrawals(source_type);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);
CREATE INDEX IF NOT EXISTS idx_withdrawals_deleted_at ON withdrawals(deleted_at);

