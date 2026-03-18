# POS Accounting Sync Reconcile

Date: 2026-03-13

Scope:
- POS backend: `possystem`
- Accounting target: `apifitness`

Execution update:
- safe batch `15`, `18`, `19`, `20`-`36` is now `synced`
- posted POS journals created in `apifitness` for sales `20`-`36` as journal IDs `76`-`92`
- remaining unsynced POS sales are now only `17`, `37`, `38`, `39`

## Pending Record IDs

### POS sale records still `pending` in database `pos`

IDs:
- `15`
- `17`
- `18`
- `19`
- `20`
- `21`
- `22`
- `23`
- `24`
- `25`
- `26`
- `27`
- `28`
- `29`
- `30`
- `31`
- `32`
- `33`
- `34`
- `35`
- `36`
- `37`
- `38`
- `39`
- `46`
- `47`
- `48`

Important split:
- `24` old records (`15`, `17`-`39`) do not persist `accounting_idempotency_key` in the row yet, so they are legacy backlog.
- `3` recent records (`46`, `47`, `48`) already have `accounting_idempotency_key` and local journal entries, but still do not exist in `apifitness.tbjournal`.

### Membership payments still pending in database `possystem`

IDs:
- `36`
- `37`
- `38`
- `39`
- `40`
- `41`
- `42`

These are workflow pending items, not POS accounting sync retries. They still need business review to be approved or cancelled.

## Findings

### 1. Historical POS backlog exists before current idempotency persistence

Older pending rows were created without a stored `accounting_idempotency_key`. The current listing layer falls back to `pos:sale:<id>`, but the rows themselves were never retried after the newer sync metadata was added.

### 2. Transaction `46` was created before the currently running POS server started

Observed times:
- transaction `46` created at `2026-03-13 19:27:17`
- current `possystem` process started at `2026-03-13 19:37:56`

Inference:
- record `46` almost certainly came from an older runtime and was left pending before the current server restarted.

### 3. Transactions `47` and `48` were created after the current POS server started, but remained in initial `pending`

Observed times:
- transaction `47` created at `2026-03-13 20:38:24`
- transaction `48` created at `2026-03-13 20:40:11`
- current `possystem` process started at `2026-03-13 19:37:56`

Observed state:
- both rows have local `journal_entry_id`
- both rows have `accounting_idempotency_key`
- both rows have no `accounting_synced_at`
- both rows have empty `accounting_sync_error`
- neither row exists in `apifitness.tbjournal`

Inference:
- the rows were left in the initial `pending` state without a completed sync result being written back.
- because `possystem` writes logs only to stdout and no persistent log file exists, the exact runtime failure point for `47` and `48` is not recoverable from disk.
- the most practical recovery path is manual reconcile/retry.

### 4. Runtime code already contains the sync implementation

The currently running temporary `go run` binary contains:
- `payment_method bayarnanti must be saved as draft first and cannot post accounting directly`
- `Accounting sync retried successfully`
- `APIFITNESS_API_URL is not configured`

So the active server binary is not missing the accounting sync feature entirely.

### 5. Legacy sale journals still use obsolete account IDs

Observed legacy IDs in failed or still-unretried sales:
- `1100` -> old cash asset account
- `1104` -> old QRIS asset account
- `4100` -> old product sales revenue account

Target chart that exists in `apifitness.tbaccounts`:
- `101` -> Cash
- `102` -> QRIS
- `402` -> POS Revenue

Practical implication:
- retry will keep failing with `one or more account IDs are invalid` until the local POS journal lines are remapped.

### 6. Outlet `17` sale revenue mapping is currently wrong

Observed mapping:
- `outlet_id = 17`
- `transaction_type = sale`
- `purpose = sales`
- `account_id = 101`

This points a sale revenue line to an asset account instead of `402` (`POS Revenue`).

Practical implication:
- even when sync succeeds, the posted journal can still be semantically wrong for outlet `17` unless the mapping is fixed.

### 7. Wrong outlet `17` mapping has already affected posted journals

Observed posted journals in `apifitness`:
- journal `70` for sale `47`
- journal `71` for sale `48`
- journal `72` for sale `46`

Observed bad lines:
- sale `47`: debit `101`, credit `101`
- sale `48`: debit `102`, credit `101`
- sale `46`: debit `1106`, credit `101`

Practical implication:
- these three journals are no longer a sync backlog problem
- they are now an accounting correction problem and need manual reversal/repost after the outlet mapping is fixed

### 8. Sale `17` is a zero-value edge case

Observed state:
- sale `17` total = `0`
- both legacy journal lines carry zero debit and zero credit

Practical implication:
- remapping account IDs alone will not make this sale postable
- it should be reviewed manually instead of included in a blind bulk retry

## Reconcile Script

File:
- `cmd/reconcile_accounting_sync/main.go`

Purpose:
- dry-run list of pending/failed accounting sync records
- safe targeted retry by explicit ID
- optional bulk retry with `-force-all`

Examples:

```bash
go run ./cmd/reconcile_accounting_sync -record-type sale -status pending
go run ./cmd/reconcile_accounting_sync -record-type sale -status pending -ids 47,48
go run ./cmd/reconcile_accounting_sync -record-type sale -status pending -ids 47,48 -apply
go run ./cmd/reconcile_accounting_sync -record-type sale -status pending -outlet 17 -apply -force-all
```

Operational caution:
- do not bulk retry `bayarnanti` sales unless those sales are confirmed ready to post into accounting
- for mixed pending sales, prefer targeted `-ids` first
- exclude sale `17` from normal retry batches unless its zero-value journal is corrected manually

## Legacy Journal Remap

File:
- `cmd/remap_legacy_sale_accounts/main.go`

Purpose:
- dry-run the legacy journal lines that still use `1100`, `1104`, or `4100`
- remap them to the current chart before retrying sync

Examples:

```bash
go run ./cmd/remap_legacy_sale_accounts -outlet 17 -status failed
go run ./cmd/remap_legacy_sale_accounts -ids 20,21,22,23 -apply
```

SQL repair for outlet `17` live mapping:
- `scripts/sql/fix_outlet_17_sale_mapping.sql`

## Hardening

Code changes added:
- mapping validation now blocks semantically invalid combinations such as `sale:sales -> asset`
- sale settlement mapping now validates `cash/qris/transfer` against asset-category accounts at runtime

Relevant files:
- `services/account_mapping_service.go`
- `controllers/account_controller.go`
