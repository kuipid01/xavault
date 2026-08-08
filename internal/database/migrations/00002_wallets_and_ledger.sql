-- +goose Up
-- Baseline domain migration. Idempotent for adoption by legacy databases.

CREATE TABLE IF NOT EXISTS wallets (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    available_balance bigint NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
    locked_balance bigint NOT NULL DEFAULT 0 CHECK (locked_balance >= 0),
    ledger_balance bigint NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'NGN',
    paystack_customer_code varchar(120),
    dedicated_account_id varchar(80),
    dedicated_account_number varchar(20),
    dedicated_account_name varchar(180),
    dedicated_bank_name varchar(160),
    dedicated_bank_slug varchar(80),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);

CREATE TABLE IF NOT EXISTS wallet_deposits (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    reference varchar(100) NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    currency text NOT NULL DEFAULT 'NGN',
    status text NOT NULL DEFAULT 'pending',
    provider text NOT NULL DEFAULT 'paystack',
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_deposits_reference ON wallet_deposits(reference);
CREATE INDEX IF NOT EXISTS idx_wallet_deposits_user_id ON wallet_deposits(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_deposits_wallet_id ON wallet_deposits(wallet_id);

CREATE TABLE IF NOT EXISTS wallet_ledgers (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    wallet_id uuid NOT NULL REFERENCES wallets(id),
    currency text NOT NULL DEFAULT 'NGN',
    reference text NOT NULL,
    type text NOT NULL,
    source text NOT NULL,
    source_id text,
    amount bigint NOT NULL CHECK (amount > 0),
    balance_before bigint NOT NULL,
    balance_after bigint NOT NULL,
    description text,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_ledgers_reference ON wallet_ledgers(reference);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_user_id ON wallet_ledgers(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_wallet_id ON wallet_ledgers(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_type ON wallet_ledgers(type);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_source ON wallet_ledgers(source);

