# Xavault V1 operations runbook

## Recovery objectives

- Recovery point objective (RPO): 15 minutes.
- Recovery time objective (RTO): 60 minutes.
- Run encrypted PostgreSQL backups at least every 15 minutes and retain daily
  copies for 35 days in a separate account/region with object lock enabled.
- The encryption key must be held outside the backup storage account.

## Backup and restore

Run `scripts/backup_database.sh` from the production scheduler with
`DATABASE_URL`, `BACKUP_ENCRYPTION_KEY`, and an upload step to immutable object
storage. A local file alone is not a backup.

Before launch and quarterly thereafter:

1. Provision an isolated PostgreSQL restore-drill database.
2. Download a recent encrypted backup and its checksum.
3. Set `RESTORE_DATABASE_URL` only to that isolated database.
4. Run `scripts/restore_drill.sh BACKUP_FILE`.
5. Start the API against the drill database, run reconciliation, and record
   start time, completion time, backup timestamp, row counts, and findings.
6. Destroy the drill environment and retain the signed exercise record.

## Financial incident response

Critical reconciliation findings automatically set
`reconciliation_controls.processing_halted`. State-changing financial endpoints
then return HTTP 503 while read-only history remains available.

Inspect with `GET /api/v1/admin/reconciliation/`, using `X-Admin-Key` and a
named `X-Operator-ID`. Correct the root cause, rerun reconciliation, and only
then call `POST /api/v1/admin/reconciliation/resume` with a written reason.
Every resume is recorded in `reconciliation_operator_actions`.

Alert on structured events including `financial_processing_halted`,
`deposit_amount_mismatch`, `withdrawal_reconcile_provider_error`,
`PAYSTACK_WEBHOOK` retry/dead events, HTTP 5xx rates, and authentication 429s.

## Secret rotation

Keep current and previous JWT verification keys during a bounded rotation
window, issue all new tokens with the new key, then revoke the previous key
after the maximum token lifetime. Rotate Paystack, Brevo, database, backup, and
admin keys through the deployment secret manager, never `.env` committed to
source control. Exercise logout-all-devices after JWT or refresh-token exposure.
