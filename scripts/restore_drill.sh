#!/usr/bin/env sh
set -eu

: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required and must target an isolated drill database}"
: "${BACKUP_ENCRYPTION_KEY:?BACKUP_ENCRYPTION_KEY is required}"

encrypted_file="${1:?usage: restore_drill.sh path/to/backup.dump.enc}"
checksum_file="$encrypted_file.sha256"
test -f "$checksum_file"
sha256sum -c "$checksum_file"

plain_file="$(mktemp)"
trap 'rm -f "$plain_file"' EXIT
openssl enc -d -aes-256-cbc -pbkdf2 \
  -in "$encrypted_file" -out "$plain_file" \
  -pass env:BACKUP_ENCRYPTION_KEY

pg_restore --clean --if-exists --no-owner --no-acl \
  --dbname "$RESTORE_DATABASE_URL" "$plain_file"

psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
SELECT COUNT(*) AS wallet_count FROM wallets;
SELECT COUNT(*) AS ledger_count FROM wallet_ledgers;
SELECT COUNT(*) AS deposit_count FROM wallet_deposits;
SELECT COUNT(*) AS withdrawal_count FROM withdrawals;
SELECT CASE WHEN COUNT(*) = 0 THEN 1 ELSE 1/0 END
FROM wallets WHERE available_balance < 0 OR locked_balance < 0 OR ledger_balance < 0;
SQL
