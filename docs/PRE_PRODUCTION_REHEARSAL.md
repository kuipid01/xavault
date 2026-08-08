# XAVAULT V1 pre-production rehearsal

This is the human-run release rehearsal for the mobile application, Go API,
PostgreSQL, Paystack test environment, Brevo email, background workers, and
operational controls. It uses Paystack test mode only. No real money should
move.

## 1. What has been hardened

| Phase | Result | Evidence |
|---|---|---|
| Database foundation | Goose owns the production schema; fresh and legacy databases are tested; production AutoMigrate is disabled | `cmd/migrate`, migrations `00001`–`00010`, database CI |
| Recovery | Encrypted verified backups, guarded restore drill, systemd schedule, RTO/RPO procedure | `scripts/backup_database.sh`, `scripts/restore_drill.sh`, recovery CI |
| Financial correctness | Reconciliation halt/resume, duplicate deposit, mismatch, withdrawal completion/reversal, webhook recovery, refresh/logout tests | integration suites and financial-correctness CI |
| Operational alerting | Durable outbox, Brevo destination, redaction, deduplication, retry/dead state, multi-destination contract | `internal/alerts`, migration `00009` |
| Shared rate limiting | PostgreSQL-backed account/IP/device counters shared by every API instance; fail-closed behavior; alert trigger | middleware store, migration `00010` |

The remaining VPS-only release evidence is a scheduled-backup restore and a
production-like HTTPS deployment rehearsal.

## 2. Absolute safety gates

Stop immediately if any gate fails.

- The backend Paystack secret starts with `sk_test_`, never `sk_live_`.
- Any Paystack public key used by checkout starts with `pk_test_`.
- The Paystack dashboard is visibly in **Test Mode**.
- Only Paystack test cards/accounts from the official test-payments page are used.
- The rehearsal database is disposable and is not the production database.
- The public tunnel targets only the local rehearsal API.
- Secrets are never placed in the mobile repository. The mobile app receives
  only the backend URL; the Paystack secret remains on the server.

Paystack states that test mode does not involve real transactions or
settlements. Test and live keys are separate and have `*_test_*` and `*_live_*`
prefixes respectively.

## 3. Local topology

```text
Physical phone / Expo Go
        |
        | HTTP over local Wi-Fi
        v
Go API at http://<computer-LAN-IP>:8080
        |
        +---- PostgreSQL Docker container
        |
        +---- Paystack test API
        |
        +---- Brevo API

Paystack test webhook
        |
        | public HTTPS tunnel
        v
https://<temporary-host>/api/v1/webhooks/paystack
        |
        v
localhost:8080
```

The phone uses the computer's LAN IP. Paystack uses the public HTTPS tunnel
because Paystack cannot send webhooks to a localhost URL.

## 4. One-time preparation

Required software:

- Docker Desktop
- Go version from `go.mod`
- Node.js/npm
- Expo Go or an Android/iOS development build
- An HTTPS tunnel such as Cloudflare Tunnel or ngrok
- Paystack account in test mode

In the Paystack test dashboard, set the webhook URL to:

```text
https://<temporary-public-host>/api/v1/webhooks/paystack
```

Ngrok is installed on this workstation. After the API is healthy, start it in a
separate terminal and leave that terminal open for the entire payment test:

```powershell
ngrok http 8080
```

Copy the HTTPS forwarding address shown by ngrok. Opening
`https://<ngrok-host>/health` in a browser must return the same healthy response
as localhost before registering the webhook URL in Paystack.

The endpoint must return HTTP 200. Do not add authentication to this route;
the backend authenticates Paystack using the `X-Paystack-Signature` HMAC.

## 5. Backend configuration

Use `C:\Users\User\dev\xavault\.env`. Confirm these values without printing
their secrets:

```env
APP_ENV=development
APP_PORT=8080
AUTO_MIGRATE=false
PAYSTACK_SECRET_KEY=sk_test_...
PAYSTACK_BASE_URL=https://api.paystack.co
BREVO_SENDER_NAME=XAVAULT Operations
BREVO_SENDER_EMAIL=<verified Brevo sender>
ALERT_EMAIL_TO=kuipid01@gmail.com
```

Set `APP_LOCAL_URL` to the temporary HTTPS tunnel if Paystack uses the callback
URL returned during transaction initialization.

Start PostgreSQL and prepare the schema:

```powershell
cd C:\Users\User\dev\xavault
docker compose up -d postgres
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/api
```

In a second terminal:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

Pass: migration status has no pending migration and health returns HTTP 200.

## 6. Mobile configuration

Find the computer's active Wi-Fi IPv4 address with `ipconfig`. Then update
`C:\Users\User\dev\xacvault\.env`:

```env
EXPO_PUBLIC_API_URL=http://<computer-LAN-IP>:8080/api/v1/
```

The phone and computer must be on the same network. Restart Expo after every
environment change:

```powershell
cd C:\Users\User\dev\xacvault
npm install
npx expo start --clear
```

Before opening the app, visit this URL from the phone browser:

```text
http://<computer-LAN-IP>:8080/health
```

Pass: the phone displays the healthy response. If it does not, check Windows
Firewall, network isolation, the IP address, and whether the API listens on
port 8080.

## 7. Rehearsal identity and evidence

Use a new test email alias so old tokens and database state cannot hide a bug:

```text
kuipid01+xavault-rehearsal-YYYYMMDD@gmail.com
```

Record:

- rehearsal date and operator;
- backend commit hash and mobile commit hash;
- phone model and OS;
- Expo/development-build version;
- temporary webhook hostname;
- screenshots for every numbered flow;
- deposit, withdrawal, and reconciliation references;
- any unexpected UI message, HTTP status, or log event.

## 8. Manual end-to-end flow

### Flow 1 — Registration, OTP, login, and restart

1. Register the new rehearsal identity.
2. Receive the OTP through Brevo and verify the account.
3. Log in with the new password.
4. Confirm the home screen and zero wallet balance load.
5. Force-close the app and reopen it.

Pass:

- registration creates only one user and one wallet;
- OTP works once and an invalid OTP is rejected;
- login succeeds without exposing tokens in logs;
- reopening restores the session using secure storage/refresh;
- wallet available, locked, and ledger balances are all zero.

### Flow 2 — Successful Paystack test deposit

1. From Home, select **Deposit Money**.
2. Enter NGN 500, the current mobile minimum.
3. Continue to Paystack checkout.
4. Use Paystack's successful reusable test card:
   - card: `4084 0840 8408 4081`
   - any future expiry;
   - CVV `408`.
5. Return to the app and allow verification to finish.
6. Open Transactions and copy the deposit reference.
7. Pull to refresh Home and Transactions.

Pass:

- the deposit begins as pending;
- the signed `charge.success` webhook is stored and processed;
- available and ledger balances increase by exactly NGN 500;
- locked balance remains unchanged;
- Transactions contains exactly one deposit ledger entry;
- repeated refresh/verification does not add money again.

Database evidence:

```powershell
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT reference, amount, status FROM wallet_deposits ORDER BY created_at DESC LIMIT 5;"
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT reference, type, amount, balance_before, balance_after FROM wallet_ledgers ORDER BY created_at DESC LIMIT 10;"
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT provider_event_id, status, attempts, resource_reference FROM paystack_events ORDER BY created_at DESC LIMIT 10;"
```

Amounts in PostgreSQL are kobo: NGN 500 is `50000`.

### Flow 3 — Declined deposit

1. Start another NGN 500 deposit.
2. Use Paystack's declined test card:
   - card: `4084 0800 0000 5408`;
   - any future expiry;
   - CVV `001`.
3. Return to the app and select verify if offered.

Pass:

- the UI does not say that money was credited;
- wallet balances do not change;
- no successful credit ledger is created;
- the pending/failed payment can be retried, continued, or dismissed clearly.

### Flow 4 — Interrupted/uncertain deposit

1. Start a new deposit but close checkout before completing it.
2. Force-close and reopen the app.
3. Confirm the pending-payment modal/banner reappears.
4. Select **Verify again**; it must remain pending/not confirmed.
5. Continue the same payment and complete it with the successful test card.

Pass:

- interruption is never shown as a successful payment;
- the original reference is reused when continuing;
- eventual success credits exactly once;
- dismissing the UI does not fabricate a successful deposit.

### Flow 5 — Plan and envelope allocation

1. Create a spending plan for less than or equal to the available wallet funds.
2. Confirm/fund the plan.
3. Create one top-level envelope.
4. Create a child envelope within the parent's amount.
5. Attempt an allocation exceeding the parent or plan amount.

Pass:

- funding moves the correct amount from available to locked;
- ledger balance remains consistent;
- valid parent/child totals are displayed correctly;
- over-allocation is rejected;
- refreshing or reopening does not allocate twice.

### Flow 6 — Withdrawal PIN protection

Use a disposable rehearsal identity for the lockout subsection.

1. Set a withdrawal PIN.
2. Preview a withdrawal.
3. Enter an incorrect PIN twice.
4. Attempt the correct PIN after the second failure.
5. Complete the PIN-reset email flow.

Pass:

- the first invalid PIN fails without moving money;
- the second invalid PIN locks withdrawals;
- the correct PIN cannot bypass the lock;
- reset clears the lock only after the valid OTP flow;
- no transfer is initiated during invalid/locked attempts.

### Flow 7 — Paystack test withdrawal

Only perform this after Flow 5 and PIN reset are correct. In Paystack test mode,
create/use a test transfer recipient. Paystack documents that Nigerian test-mode
merchants may use a regular bank account; its published Zenith transfer example
uses account `0000000000` and bank code `057`.

1. Save the test bank account/recipient.
2. Preview a small withdrawal from the funded plan.
3. Confirm using the correct PIN.
4. Keep the app open until the transfer reaches its final test status.
5. Refresh Transactions and the plan.

Pass:

- one local withdrawal reference and one provider transfer are created;
- while pending/processing, funds cannot be spent twice;
- success creates exactly one withdrawal debit ledger;
- retrying the screen/request does not initiate a second transfer;
- a test reversal restores funds exactly once and is labelled as reversed.

If Paystack test mode does not expose a reversal control for the transaction,
record the limitation and rely on the passing automated reversal integration
test. Never simulate a reversal by editing wallet balances manually.

### Flow 8 — Logout and refresh lifecycle

1. Log out from the Profile screen.
2. Reopen the app and attempt to visit a protected screen.
3. Log in again.
4. Exercise **logout all devices** if exposed in the UI/API.
5. Try the previous session on another emulator/device.

Pass:

- logout returns to authentication;
- a revoked refresh token cannot create a new access token;
- logout-all invalidates the stored session;
- login creates a new usable session;
- tokens never appear in application logs.

### Flow 9 — Shared rate limit and alert

Use a dummy email, not the main rehearsal account. Submit an incorrect login at
least nine times within 15 minutes from the app or an API client.

Pass:

- the ninth attempt returns HTTP 429 with `Retry-After`;
- changing `X-Device-ID` does not bypass the account/IP limit;
- restarting one API instance does not clear the PostgreSQL counter;
- an `authentication_rate_limit_exceeded` alert enters the alert outbox;
- Brevo sends at most one matching email during the 15-minute dedupe window.

Evidence:

```powershell
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT policy, attempt_count, expires_at FROM authentication_rate_limits ORDER BY updated_at DESC LIMIT 10;"
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT event_type, severity, created_at FROM operational_alerts ORDER BY created_at DESC LIMIT 10;"
docker exec salary_vault_postgres psql -U salary_vault -d salary_vault -c "SELECT destination, status, attempts, last_error FROM operational_alert_deliveries ORDER BY created_at DESC LIMIT 10;"
```

### Flow 10 — Reconciliation and safety halt

Do not corrupt the rehearsal database manually while using the mobile flow.
First run the protected reconciliation endpoint and confirm a clean result. Use
the automated integration test—not the shared manual database—to rehearse an
intentional invariant violation and resume audit.

Pass:

- clean reconciliation reports zero critical findings;
- wallet equation holds: `available + locked = ledger`;
- each successful deposit and paid/reversed withdrawal has the expected ledger;
- no duplicate references exist;
- the tested corrupt-state scenario halts writes and sends the Brevo alert;
- operator resume is audited with operator ID and reason.

## 9. Restart and recovery behavior

1. With no request currently executing, stop the API.
2. Leave PostgreSQL running.
3. Restart the API.
4. Reopen the mobile app and refresh all screens.
5. Confirm pending webhook, alert, and reconciliation workers resume.

Pass: balances and histories are unchanged, the session refreshes normally,
pending durable work is not lost, and no duplicate credit/debit appears.

## 10. Automated release checks

Backend:

```powershell
cd C:\Users\User\dev\xavault
go test ./...
$env:XAVAULT_FINANCIAL_TEST_DSN="host=localhost port=55433 user=xavault password=<test-password> dbname=xavault_financial_test sslmode=disable"
make financial-test
go run ./cmd/migrate status
```

Mobile:

```powershell
cd C:\Users\User\dev\xacvault
npx tsc --noEmit
npm run lint
npx expo-doctor
```

All commands must pass or have a written, reviewed exception before building.
At the latest local baseline, TypeScript passes and mobile lint reports two
warnings (a React Hook dependency warning in envelope creation and an array-type
style warning in change-email). Resolve both before checking this gate.

## 11. Final launch gate

Do not build/push the public production release until all are true:

- [ ] Every manual flow above passed on Android.
- [ ] Every applicable flow passed on iOS or an explicit iOS limitation is accepted.
- [ ] Expo Web was tested separately if it will be shipped.
- [ ] Paystack test webhook delivery and signature verification passed.
- [ ] Successful, declined, and interrupted deposits behaved correctly.
- [ ] Plan/envelope allocation and withdrawal PIN lockout passed.
- [ ] Test transfer completed without duplicate initiation.
- [ ] Logout, refresh rotation, and logout-all passed.
- [ ] Shared rate limiting survived API restart and produced an alert.
- [ ] Reconciliation was clean after all financial flows.
- [ ] The admin received a Brevo assurance/critical alert.
- [ ] Backend tests, migration tests, TypeScript, lint, and Expo Doctor passed.
- [ ] A scheduled VPS backup was restored and measured RTO/RPO were recorded.
- [ ] Production uses HTTPS, `AUTO_MIGRATE=false`, and only live deployment secrets.
- [ ] Test keys are replaced with live keys only after the complete test-mode sign-off.

## 12. Failure rule

If any balance, ledger, webhook, withdrawal, or reconciliation result is
ambiguous, stop the rehearsal. Save the reference, screenshots, API logs, and
relevant database rows. Do not retry by creating a second transaction until the
original reference has been inspected and reconciled.
