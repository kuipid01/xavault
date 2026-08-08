-- +goose Up
-- Baseline domain migration. Idempotent for adoption by legacy databases.

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    password text NOT NULL,
    first_name text NOT NULL,
    last_name text NOT NULL,
    mail_verified boolean NOT NULL DEFAULT false,
    country text NOT NULL,
    phone_number text NOT NULL,
    nickname text,
    date_of_birth timestamptz,
    address text,
    profile_image_url text,
    profile_image_public_id text,
    email_verified_at timestamptz,
    verification_token text,
    verification_token_expires_at timestamptz,
    password_reset_token text,
    password_reset_token_expires_at timestamptz,
    refresh_token text,
    withdrawal_pin_hash text,
    withdrawal_pin_failed_attempts bigint NOT NULL DEFAULT 0,
    withdrawal_pin_locked_at timestamptz,
    withdrawal_pin_reset_otp text,
    withdrawal_pin_reset_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_nickname ON users (nickname) WHERE nickname IS NOT NULL;

CREATE TABLE IF NOT EXISTS temporary_users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    verified boolean NOT NULL DEFAULT false,
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporary_users_email ON temporary_users (lower(email));

CREATE TABLE IF NOT EXISTS biometric_devices (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id varchar(128) NOT NULL,
    device_name varchar(128) NOT NULL,
    platform varchar(32) NOT NULL,
    public_key text NOT NULL,
    key_algorithm varchar(16) NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_biometric_devices_device_id ON biometric_devices(device_id);
CREATE INDEX IF NOT EXISTS idx_biometric_devices_user_id ON biometric_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_biometric_devices_revoked_at ON biometric_devices(revoked_at);

CREATE TABLE IF NOT EXISTS biometric_challenges (
    id uuid PRIMARY KEY,
    credential_id uuid NOT NULL REFERENCES biometric_devices(id) ON DELETE CASCADE,
    challenge_hash char(64) NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_biometric_challenges_credential_id ON biometric_challenges(credential_id);
CREATE INDEX IF NOT EXISTS idx_biometric_challenges_expires_at ON biometric_challenges(expires_at);

CREATE TABLE IF NOT EXISTS security_questions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question varchar(255) NOT NULL,
    answer_hash varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_security_questions_user_id ON security_questions(user_id);

