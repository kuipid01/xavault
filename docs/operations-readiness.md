# Xavault V1 operations runbook

## Recovery objectives

- Recovery point objective (RPO): 15 minutes.
- Recovery time objective (RTO): 60 minutes.
- Run encrypted PostgreSQL backups at least every 15 minutes and retain daily
  copies for 35 days in a separate account/region with object lock enabled.
- The encryption key must be held outside the backup storage account.

## Backup and restore

Install `deploy/systemd/xavault-backup.service` and
`deploy/systemd/xavault-backup.timer` on the VPS. Store the populated environment
file at `/etc/xavault/backup.env` with mode `0600`, then enable the schedule:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now xavault-backup.timer
systemctl list-timers xavault-backup.timer
```

The backup script writes the encrypted dump atomically, verifies that the dump
catalog is readable before encryption, emits a portable checksum and metadata,
and removes complete local artifacts older than `BACKUP_RETENTION_DAYS`.
Upload `.enc`, `.sha256`, and `.meta` files to immutable object storage. A local
file alone is not a backup. Alert if no successful metadata file has arrived
within 30 minutes.

Before launch and quarterly thereafter:

1. Provision an isolated PostgreSQL restore-drill database.
2. Download a recent encrypted backup and its checksum.
3. Set `RESTORE_DATABASE_URL` only to that isolated database.
4. Create a database whose name ends in `_restore_drill`.
5. Set `RESTORE_CONFIRMATION=isolated-drill` and run
   `sh scripts/restore_drill.sh BACKUP_FILE`.
6. The script verifies the checksum, decrypts the dump, validates its catalog,
   restores with fail-fast behavior, and checks tables, balances, and duplicate
   financial references.
7. Start the API with `AUTO_MIGRATE=false`, run reconciliation, and record
   start time, completion time, backup timestamp, row counts, and findings.
8. Destroy the drill environment and retain the signed exercise record.

### Restore exercise record

Record: exercise owner, backup creation time, restore start/end time, measured
RPO, measured RTO, encrypted object checksum, source/target PostgreSQL versions,
restored row counts, reconciliation run ID/findings, API health result, and any
corrective actions. A drill passes only when RPO is at most 15 minutes, RTO is at
most 60 minutes, verification succeeds, reconciliation has no unresolved
critical findings, and the API health check passes.

## Financial incident response

Critical reconciliation findings automatically set
`reconciliation_controls.processing_halted`. State-changing financial endpoints
then return HTTP 503 while read-only history remains available.

Inspect with `GET /api/v1/admin/reconciliation/`, using `X-Admin-Key` and a
named `X-Operator-ID`. Correct the root cause, rerun reconciliation, and only
then call `POST /api/v1/admin/reconciliation/resume` with a written reason.
Every resume is recorded in `reconciliation_operator_actions`.

## Recovery integration tests

Use a dedicated, disposable PostgreSQL database. The tests deliberately
exercise `FOR UPDATE SKIP LOCKED`, database-clock leases, transaction rollback,
and replay after a committed financial mutation:

```sh
TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/webhooks
```

Never point `TEST_DATABASE_URL` at production or a shared staging database.

## Financial correctness tests

The reconciliation, deposit-credit, withdrawal-completion, and reversal tests
run against a real disposable PostgreSQL database built exclusively from the
versioned Goose migrations. The database name must contain `financial_test`;
the harness refuses to reset any other database.

```sh
XAVAULT_FINANCIAL_TEST_DSN='postgres://...' make financial-test
```

These tests verify clean and corrupt reconciliation outcomes, processing-halt
and audited resume behavior, provider timeout and ambiguous verification safety,
amount-mismatch rejection, signed webhook persistence and deposit allocation,
concurrent duplicate callbacks, duplicate withdrawal completion and reversal,
refresh-token rotation/replay rejection, logout, and logout-all-devices. CI also
runs the same suite with Go's race detector. Because each Go package recreates
the disposable schema, keep package execution serial with `-p 1`; concurrency
is exercised inside the relevant test cases.

Alert on structured events including `financial_processing_halted`,
`deposit_amount_mismatch`, `withdrawal_reconcile_provider_error`,
`PAYSTACK_WEBHOOK` retry/dead events, HTTP 5xx rates, and authentication 429s.

## Operational alert delivery

Production alerts use a durable PostgreSQL outbox and provider-neutral
destinations. Brevo email is the first configured destination; additional
destinations such as Sentry can be registered without changing financial
services. Configure:

```sh
BREVO_API_KEY=...
BREVO_SENDER_NAME=XAVAULT Operations
BREVO_SENDER_EMAIL=alerts@example.com
ALERT_EMAIL_TO=operator@example.com
```

The API stores alerts before attempting delivery. A background worker sends due
deliveries every 30 seconds, retries failures with bounded exponential delays,
and marks a delivery dead after eight attempts. Identical fingerprints are
suppressed for 15 minutes. Metadata keys containing token, secret, password,
PIN, or payload are redacted before storage. Brevo failure never rolls back a
financial transaction.

Monitor `operational_alert_deliveries` for `retrying` and `dead` rows. Before
launch, deliberately trigger a reconciliation violation in an isolated database
and confirm that the configured mailbox receives the critical alert.

## Secret rotation

Keep current and previous JWT verification keys during a bounded rotation
window, issue all new tokens with the new key, then revoke the previous key
after the maximum token lifetime. Rotate Paystack, Brevo, database, backup, and
admin keys through the deployment secret manager, never `.env` committed to
source control. Exercise logout-all-devices after JWT or refresh-token exposure.
