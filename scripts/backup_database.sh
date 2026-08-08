#!/usr/bin/env sh
set -eu
umask 077

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_ENCRYPTION_KEY:?BACKUP_ENCRYPTION_KEY is required}"

for command_name in pg_dump openssl sha256sum; do
  command -v "$command_name" >/dev/null 2>&1 || {
    printf 'required command not found: %s\n' "$command_name" >&2
    exit 1
  }
done

backup_dir="${BACKUP_DIR:-./backups}"
retention_days="${BACKUP_RETENTION_DAYS:-35}"
case "$backup_dir" in ""|"/"|".") printf 'BACKUP_DIR must be a dedicated directory\n' >&2; exit 1;; esac
case "$retention_days" in *[!0-9]*|"") printf 'BACKUP_RETENTION_DAYS must be a positive integer\n' >&2; exit 1;; esac

mkdir -p "$backup_dir"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
base_name="xavault-$timestamp.dump"
plain_file="$backup_dir/$base_name.tmp"
encrypted_tmp="$backup_dir/$base_name.enc.tmp"
encrypted_file="$backup_dir/$base_name.enc"
checksum_file="$encrypted_file.sha256"
metadata_file="$encrypted_file.meta"
trap 'rm -f "$plain_file" "$encrypted_tmp"' EXIT HUP INT TERM

started_epoch="$(date +%s)"
pg_dump --format=custom --no-owner --no-acl "$DATABASE_URL" --file "$plain_file"
pg_restore --list "$plain_file" >/dev/null
openssl enc -aes-256-cbc -pbkdf2 -salt \
  -in "$plain_file" -out "$encrypted_tmp" \
  -pass env:BACKUP_ENCRYPTION_KEY
mv "$encrypted_tmp" "$encrypted_file"
rm -f "$plain_file"

(
  cd "$backup_dir"
  sha256sum "$(basename "$encrypted_file")" > "$(basename "$checksum_file")"
)
finished_epoch="$(date +%s)"
size_bytes="$(wc -c < "$encrypted_file" | tr -d ' ')"
checksum="$(cut -d ' ' -f 1 "$checksum_file")"
{
  printf 'created_at_utc=%s\n' "$timestamp"
  printf 'duration_seconds=%s\n' "$((finished_epoch - started_epoch))"
  printf 'encrypted_size_bytes=%s\n' "$size_bytes"
  printf 'sha256=%s\n' "$checksum"
  printf 'pg_dump_version=%s\n' "$(pg_dump --version)"
  printf 'encryption=aes-256-cbc-pbkdf2\n'
} > "$metadata_file"

# Retention applies only to complete Xavault backup artifacts in this dedicated directory.
find "$backup_dir" -maxdepth 1 -type f \
  \( -name 'xavault-*.dump.enc' -o -name 'xavault-*.dump.enc.sha256' -o -name 'xavault-*.dump.enc.meta' \) \
  -mtime "+$retention_days" -delete

printf '%s\n' "$encrypted_file"
