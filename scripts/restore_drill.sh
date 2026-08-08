#!/usr/bin/env sh
set -eu
umask 077

: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required}"
: "${BACKUP_ENCRYPTION_KEY:?BACKUP_ENCRYPTION_KEY is required}"
: "${RESTORE_CONFIRMATION:?set RESTORE_CONFIRMATION=isolated-drill}"
test "$RESTORE_CONFIRMATION" = "isolated-drill" || {
  printf 'RESTORE_CONFIRMATION must equal isolated-drill\n' >&2
  exit 1
}

for command_name in psql pg_restore openssl sha256sum; do
  command -v "$command_name" >/dev/null 2>&1 || {
    printf 'required command not found: %s\n' "$command_name" >&2
    exit 1
  }
done

encrypted_file="${1:?usage: restore_drill.sh path/to/backup.dump.enc}"
checksum_file="$encrypted_file.sha256"
test -f "$encrypted_file" || { printf 'backup not found: %s\n' "$encrypted_file" >&2; exit 1; }
test -f "$checksum_file" || { printf 'checksum not found: %s\n' "$checksum_file" >&2; exit 1; }

database_name="$(psql "$RESTORE_DATABASE_URL" -Atv ON_ERROR_STOP=1 -c 'SELECT current_database()')"
case "$database_name" in
  *_restore_drill) ;;
  *) printf 'refusing restore: database %s must end with _restore_drill\n' "$database_name" >&2; exit 1;;
esac

existing_tables="$(psql "$RESTORE_DATABASE_URL" -Atv ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_tables WHERE schemaname='public'")"
if [ "$existing_tables" -ne 0 ] && [ "${ALLOW_OVERWRITE_RESTORE_DRILL:-false}" != "true" ]; then
  printf 'restore target is not empty; set ALLOW_OVERWRITE_RESTORE_DRILL=true only for this isolated drill database\n' >&2
  exit 1
fi

backup_dir="$(CDPATH= cd -- "$(dirname -- "$encrypted_file")" && pwd)"
backup_name="$(basename -- "$encrypted_file")"
(
  cd "$backup_dir"
  sha256sum -c "$(basename -- "$checksum_file")"
)

plain_file="$(mktemp)"
trap 'rm -f "$plain_file"' EXIT HUP INT TERM
started_epoch="$(date +%s)"
openssl enc -d -aes-256-cbc -pbkdf2 \
  -in "$backup_dir/$backup_name" -out "$plain_file" \
  -pass env:BACKUP_ENCRYPTION_KEY
pg_restore --list "$plain_file" >/dev/null
pg_restore --clean --if-exists --no-owner --no-acl \
  --exit-on-error --dbname "$RESTORE_DATABASE_URL" "$plain_file"

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$script_dir/verify_restored_database.sql"
finished_epoch="$(date +%s)"
printf 'restore_database=%s\n' "$database_name"
printf 'restore_duration_seconds=%s\n' "$((finished_epoch - started_epoch))"
printf 'restore_completed_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
