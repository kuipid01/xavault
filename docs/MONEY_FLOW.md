# Money Flow: Deposits, Plans, Envelopes, and Withdrawals

All monetary amounts are stored and transported in minor units (kobo for NGN).
The wallet has three balances:

- `available_balance`: money that can be assigned to a plan.
- `locked_balance`: money assigned to plans and envelopes.
- `ledger_balance`: total money held for the user.

## Deposit flow

1. The client creates a pending deposit with a unique reference and opens Paystack checkout.
2. Paystack sends `charge.success`, or the client asks the backend to verify the reference.
3. Both paths call the same `CreditWallet` operation.
4. One database transaction locks the deposit row and wallet row.
5. The transaction verifies that the deposit is not already successful, the Paystack amount exactly matches the pending amount, and the subscription capacity permits the credit.
6. The transaction marks the deposit successful, increases `available_balance` and `ledger_balance`, and inserts one credit ledger entry.
7. A duplicate webhook or verification sees the successful deposit and returns success without crediting again.
8. A real processing failure returns a non-200 webhook response so Paystack can retry.

Dedicated-account deposits use the same idempotent behavior. A unique deposit reference prevents duplicate rows, and the wallet row is locked while it is credited.

## Plan and envelope funding

1. Confirming a plan moves its amount from `available_balance` to `locked_balance`; `ledger_balance` does not change.
2. Envelopes allocate portions of a plan. Creating an envelope does not move wallet money again.
3. A parent plan or envelope can only expose its unboxed amount:

   `amount - child allocations - paid withdrawals - pending withdrawals`

4. Pending and processing withdrawals remain reserved without a time limit. They cannot silently become available because a provider response was delayed.
5. Plan cancellation and envelope deletion are blocked while any related withdrawal is pending or processing.

## Withdrawal flow

1. The client previews the selected plan or envelope source.
2. The backend validates the PIN, destination, source ownership, policy, and withdrawable amount inside a transaction.
3. The plan and wallet rows are locked. A pending withdrawal with a stable `WITHDRAWAL_*` reference is created, but wallet balances do not move yet.
4. The backend sends that same reference to Paystack.
5. If Paystack explicitly rejects initiation, the withdrawal becomes failed and its source reservation is released.
6. If Paystack accepts initiation, the withdrawal becomes processing. This update only applies while the record is still pending, so a fast success webhook cannot be overwritten back to processing.
7. If Paystack may have accepted but the local provider response cannot be saved, the withdrawal stays pending. It blocks another withdrawal from the same source and requires webhook completion or provider reconciliation. It is never automatically failed or retried as a new transfer.
8. On `transfer.success`, one transaction locks the withdrawal and wallet, subtracts the amount from `locked_balance` and `ledger_balance`, marks the withdrawal paid, and creates one withdrawal debit ledger entry using the original reference.
9. Duplicate success webhooks are idempotent.

## Failed and reversed transfers

- A failure before payment marks a pending or processing withdrawal failed. Wallet balances were not moved, so no compensating ledger entry is needed.
- A `transfer.reversed` received after payment performs a compensating transaction: the amount is added back to `locked_balance` and `ledger_balance`, the withdrawal becomes reversed, and a refund ledger entry is created.
- Repeated reversal webhooks are idempotent.
- Restored money returns to its original plan/envelope accounting because reversed withdrawals are excluded from paid totals.

## Client status and caching

- Pending or processing: orange.
- Paid or completed: green.
- Failed, cancelled, rejected, or reversed: red.

The withdrawal modal polls briefly after initiation. The Transactions screen polls while any visible withdrawal is non-terminal. Terminal history is cached for five minutes, then may be refreshed on remount/focus so a later provider reversal can still be discovered. A status change refreshes the wallet ledger so receipts obtain authoritative before-and-after balances.

Withdrawal history is paginated. The API accepts `page` and `limit`, caps `limit` at 100, and returns `items`, `page`, `limit`, and `total`. The client loads 50 recent records and exposes `Load older` while another page exists.

## Operational recovery

A withdrawal that remains pending must be treated as provider-uncertain, not failed. Support or a reconciliation worker should query Paystack using the original `WITHDRAWAL_*` reference and then apply the corresponding success or failure transition. Do not create a replacement transfer until the original reference has a confirmed terminal result.

For webhook monitoring, alert on repeated non-200 responses and on withdrawals that remain pending or processing beyond the normal provider window. These alerts represent recoverable work; they must not automatically release or duplicate money.
