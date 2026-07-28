#!/usr/bin/env sh
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_ENCRYPTION_KEY:?BACKUP_ENCRYPTION_KEY is required}"

backup_dir="${BACKUP_DIR:-./backups}"
mkdir -p "$backup_dir"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
plain_file="$backup_dir/xavault-$timestamp.dump"
encrypted_file="$plain_file.enc"

pg_dump --format=custom --no-owner --no-acl "$DATABASE_URL" --file "$plain_file"
openssl enc -aes-256-cbc -pbkdf2 -salt \
  -in "$plain_file" -out "$encrypted_file" \
  -pass env:BACKUP_ENCRYPTION_KEY
rm -f "$plain_file"
sha256sum "$encrypted_file" > "$encrypted_file.sha256"
printf '%s\n' "$encrypted_file"
