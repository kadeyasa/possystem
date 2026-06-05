#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POSSYSTEM_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_SOURCE_ENV="${POSSYSTEM_ROOT}/../apifitness/.env"
DEFAULT_TARGET_ENV="${POSSYSTEM_ROOT}/.env"
MASTER_SYNC_SCRIPT="${SCRIPT_DIR}/sync_legacy_accounting_accounts.sh"

SOURCE_ENV_FILE="${SOURCE_ENV_FILE:-${DEFAULT_SOURCE_ENV}}"
TARGET_ENV_FILE="${TARGET_ENV_FILE:-${DEFAULT_TARGET_ENV}}"
SOURCE_DSN="${SOURCE_DSN:-}"
TARGET_DSN="${TARGET_DSN:-}"
SOURCE_ACCOUNT_TABLE="${SOURCE_ACCOUNT_TABLE:-}"
SOURCE_DELETED_FILTER="${SOURCE_DELETED_FILTER:-}"
SOURCE_OUTLET_TABLE="${SOURCE_OUTLET_TABLE:-}"
SOURCE_OUTLET_DELETED_FILTER="${SOURCE_OUTLET_DELETED_FILTER:-}"
MODE="${MODE:-fill}"
SCOPE="${SCOPE:-all-outlets}"
SYNC_MASTER_ACCOUNTS=1
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage:
  bash scripts/seed_legacy_account_mappings.sh [options]

Options:
  --source-env PATH        Path .env source legacy database. Default: ../apifitness/.env
  --target-env PATH        Path .env target POS database. Default: ./.env
  --source-dsn DSN         Override source PostgreSQL DSN.
  --target-dsn DSN         Override target PostgreSQL DSN.
  --source-table NAME      Override source accounts table (tbaccounts or tblaccounts).
  --source-outlet-table NAME Override source outlet table (tboutlet or tbloutlet).
  --mode MODE              fill | replace. Default: fill
  --scope SCOPE            all-outlets | global-only. Default: all-outlets
  --skip-account-sync      Do not run phase 1 account master sync first.
  --dry-run                Preview selected mappings only, do not write to target DB.
  --help                   Show this help.

Rules seeded:
  - sale:cash
  - sale:qris
  - sale:transfer
  - sale:sales
  - sale:tax
  - sale:cogs
  - purchase:inventory (auto-added when found, because it pairs with COGS posting)

Notes:
  - Recommended first run: --dry-run
  - Mode fill keeps existing account_id mapping keys and only fills missing/reactivates inactive rows.
  - Mode replace overwrites existing account_id for the seeded keys.
EOF
}

log() {
  printf '[seed-account-mappings] %s\n' "$*"
}

fail() {
  printf '[seed-account-mappings] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_table() {
  local dsn="$1"
  local table_name="$2"
  local resolved

  resolved="$(psql "$dsn" -At -c "SELECT to_regclass('public.${table_name}')")"
  [[ "$resolved" == "public.${table_name}" ]] || fail "table not found: ${table_name}"
}

detect_source_account_table() {
  local dsn="$1"
  local resolved

  resolved="$(psql "$dsn" -At <<'SQL'
SELECT COALESCE(
  to_regclass('public.tbaccounts')::text,
  to_regclass('public.tblaccounts')::text,
  ''
);
SQL
)"

  case "$resolved" in
    public.tbaccounts) printf 'tbaccounts' ;;
    public.tblaccounts) printf 'tblaccounts' ;;
    *) return 1 ;;
  esac
}

detect_source_outlet_table() {
  local dsn="$1"
  local resolved

  resolved="$(psql "$dsn" -At <<'SQL'
SELECT COALESCE(
  to_regclass('public.tboutlet')::text,
  to_regclass('public.tbloutlet')::text,
  ''
);
SQL
)"

  case "$resolved" in
    public.tboutlet) printf 'tboutlet' ;;
    public.tbloutlet) printf 'tbloutlet' ;;
    *) return 1 ;;
  esac
}

detect_optional_deleted_filter() {
  local dsn="$1"
  local table_name="$2"
  local has_deleted_at

  has_deleted_at="$(psql "$dsn" -At <<SQL
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = 'public'
    AND table_name = '${table_name}'
    AND column_name = 'deleted_at'
);
SQL
)"

  if [[ "$has_deleted_at" == "t" ]]; then
    printf 'AND deleted_at IS NULL'
  else
    printf ''
  fi
}

read_env_var() {
  local env_file="$1"
  local var_name="$2"

  [[ -f "$env_file" ]] || fail "env file not found: $env_file"

  (
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
    eval "printf '%s' \"\${${var_name}:-}\""
  )
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-env)
      [[ $# -ge 2 ]] || fail "--source-env requires a value"
      SOURCE_ENV_FILE="$2"
      shift 2
      ;;
    --target-env)
      [[ $# -ge 2 ]] || fail "--target-env requires a value"
      TARGET_ENV_FILE="$2"
      shift 2
      ;;
    --source-dsn)
      [[ $# -ge 2 ]] || fail "--source-dsn requires a value"
      SOURCE_DSN="$2"
      shift 2
      ;;
    --target-dsn)
      [[ $# -ge 2 ]] || fail "--target-dsn requires a value"
      TARGET_DSN="$2"
      shift 2
      ;;
    --source-table)
      [[ $# -ge 2 ]] || fail "--source-table requires a value"
      SOURCE_ACCOUNT_TABLE="$2"
      shift 2
      ;;
    --source-outlet-table)
      [[ $# -ge 2 ]] || fail "--source-outlet-table requires a value"
      SOURCE_OUTLET_TABLE="$2"
      shift 2
      ;;
    --mode)
      [[ $# -ge 2 ]] || fail "--mode requires a value"
      MODE="$2"
      shift 2
      ;;
    --scope)
      [[ $# -ge 2 ]] || fail "--scope requires a value"
      SCOPE="$2"
      shift 2
      ;;
    --skip-account-sync)
      SYNC_MASTER_ACCOUNTS=0
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

require_command psql
require_command mktemp
require_command awk

case "$MODE" in
  fill|replace) ;;
  *) fail "invalid --mode: $MODE" ;;
esac

case "$SCOPE" in
  all-outlets|global-only) ;;
  *) fail "invalid --scope: $SCOPE" ;;
esac

if [[ -z "$SOURCE_DSN" ]]; then
  SOURCE_DSN="$(read_env_var "$SOURCE_ENV_FILE" DATABASE_DSN)"
fi

if [[ -z "$TARGET_DSN" ]]; then
  TARGET_DSN="$(read_env_var "$TARGET_ENV_FILE" DATABASE_DSN)"
fi

[[ -n "$SOURCE_DSN" ]] || fail "source DSN is empty"
[[ -n "$TARGET_DSN" ]] || fail "target DSN is empty"

require_table "$TARGET_DSN" "tblaccounts"

if [[ -z "$SOURCE_ACCOUNT_TABLE" ]]; then
  SOURCE_ACCOUNT_TABLE="$(detect_source_account_table "$SOURCE_DSN" || true)"
fi

case "$SOURCE_ACCOUNT_TABLE" in
  tbaccounts|tblaccounts) ;;
  *)
    fail "source accounts table not found. Expected public.tbaccounts or public.tblaccounts"
    ;;
esac

SOURCE_DELETED_FILTER="$(detect_optional_deleted_filter "$SOURCE_DSN" "$SOURCE_ACCOUNT_TABLE")"

if [[ -z "$SOURCE_OUTLET_TABLE" ]]; then
  SOURCE_OUTLET_TABLE="$(detect_source_outlet_table "$SOURCE_DSN" || true)"
fi

case "$SOURCE_OUTLET_TABLE" in
  tboutlet|tbloutlet) ;;
  *)
    fail "source outlet table not found. Expected public.tboutlet or public.tbloutlet"
    ;;
esac

SOURCE_OUTLET_DELETED_FILTER="$(detect_optional_deleted_filter "$SOURCE_DSN" "$SOURCE_OUTLET_TABLE")"

if [[ "$SYNC_MASTER_ACCOUNTS" -eq 1 ]]; then
  [[ -x "$MASTER_SYNC_SCRIPT" ]] || fail "master sync script not found or not executable: $MASTER_SYNC_SCRIPT"
  log "Running phase 1 account master sync first"
  bash "$MASTER_SYNC_SCRIPT" \
    --source-dsn "$SOURCE_DSN" \
    --target-dsn "$TARGET_DSN"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
MAPPING_CSV="${TMP_DIR}/mapping_seed.csv"
OUTLET_CSV="${TMP_DIR}/outlet_ids.csv"

log "Source table : $SOURCE_ACCOUNT_TABLE"
log "Source outlet table : $SOURCE_OUTLET_TABLE"
log "Selecting best legacy accounts for POS default mappings"

psql "$SOURCE_DSN" -v ON_ERROR_STOP=1 -v csv_file="$MAPPING_CSV" -v source_account_table="$SOURCE_ACCOUNT_TABLE" -v source_deleted_filter="$SOURCE_DELETED_FILTER" <<'SQL'
\copy (
  WITH accounts AS (
    SELECT
      BTRIM(COALESCE(account_id::text, '')) AS account_id,
      LOWER(BTRIM(COALESCE(account_name, ''))) AS account_name,
      LOWER(BTRIM(COALESCE(account_type, ''))) AS account_type
    FROM :"source_account_table"
    WHERE 1=1
      :source_deleted_filter
      AND BTRIM(COALESCE(account_id::text, '')) <> ''
  ),
  candidate_rules AS (
    SELECT
      'sale'::text AS transaction_type,
      'cash'::text AS purpose,
      account_id,
      CASE
        WHEN account_type = 'asset' AND account_name ~ '(petty cash|cash on hand|kas kecil)' THEN 5
        WHEN account_type = 'asset' AND account_name ~ '(^|[^a-z])(cash|kas)($|[^a-z])' THEN 10
        WHEN account_type = 'asset' AND account_name LIKE '%cash%' THEN 20
        WHEN account_type = 'asset' AND account_name LIKE '%kas%' THEN 30
        ELSE 999
      END AS priority
    FROM accounts

    UNION ALL

    SELECT
      'sale',
      'qris',
      account_id,
      CASE
        WHEN account_type = 'asset' AND account_name ~ '(^|[^a-z])qris($|[^a-z])' THEN 5
        WHEN account_type = 'asset' AND account_name LIKE '%qris%' THEN 10
        ELSE 999
      END
    FROM accounts

    UNION ALL

    SELECT
      'sale',
      'transfer',
      account_id,
      CASE
        WHEN account_type = 'asset' AND account_name ~ '(bank transfer|transfer bank)' THEN 5
        WHEN account_type = 'asset' AND account_name ~ '(^|[^a-z])transfer($|[^a-z])' THEN 10
        WHEN account_type = 'asset' AND account_name ~ '(bank|bca|bni|bri|mandiri|permata|cimb|danamon|rekening|tabungan)' THEN 20
        ELSE 999
      END
    FROM accounts

    UNION ALL

    SELECT
      'sale',
      'sales',
      account_id,
      CASE
        WHEN account_type = 'revenue' AND account_name ~ '(pos revenue|pendapatan pos|penjualan pos)' THEN 5
        WHEN account_type = 'revenue' AND account_name ~ '(penjualan|sales|revenue|pendapatan)' THEN 10
        WHEN account_type = 'revenue' THEN 30
        ELSE 999
      END
    FROM accounts
    WHERE account_name !~ '(discount|diskon|retur|return|refund|tax|pajak|ppn)'

    UNION ALL

    SELECT
      'sale',
      'tax',
      account_id,
      CASE
        WHEN account_type = 'liability' AND account_name ~ '(ppn keluaran|pajak keluaran|sales tax)' THEN 5
        WHEN account_type = 'liability' AND account_name ~ '(tax|pajak|ppn|vat)' THEN 10
        WHEN account_type = 'liability' THEN 40
        ELSE 999
      END
    FROM accounts

    UNION ALL

    SELECT
      'sale',
      'cogs',
      account_id,
      CASE
        WHEN account_type = 'expense' AND account_name ~ '(harga pokok penjualan|beban pokok penjualan)' THEN 5
        WHEN account_type = 'expense' AND account_name ~ '(^|[^a-z])hpp($|[^a-z])' THEN 10
        WHEN account_type = 'expense' AND account_name ~ '(cogs|cost of goods|harga pokok|beban pokok)' THEN 15
        ELSE 999
      END
    FROM accounts

    UNION ALL

    SELECT
      'purchase',
      'inventory',
      account_id,
      CASE
        WHEN account_type = 'asset' AND account_name ~ '(persediaan barang|inventory asset)' THEN 5
        WHEN account_type = 'asset' AND account_name ~ '(persediaan|inventory|stock|stok)' THEN 10
        ELSE 999
      END
    FROM accounts
  ),
  ranked AS (
    SELECT
      transaction_type,
      purpose,
      account_id,
      priority,
      ROW_NUMBER() OVER (
        PARTITION BY transaction_type, purpose
        ORDER BY priority ASC, account_id ASC
      ) AS row_num
    FROM candidate_rules
    WHERE priority < 999
  )
  SELECT
    transaction_type,
    purpose,
    account_id,
    priority
  FROM ranked
  WHERE row_num = 1
  ORDER BY transaction_type ASC, purpose ASC
) TO :'csv_file' WITH CSV HEADER
SQL

[[ -s "$MAPPING_CSV" ]] || fail "no candidate mappings exported"

required_keys=(
  "sale:cash"
  "sale:qris"
  "sale:transfer"
  "sale:sales"
  "sale:tax"
  "sale:cogs"
)

exported_keys="$(
  awk -F',' 'NR > 1 { gsub(/\r/, "", $1); gsub(/\r/, "", $2); print $1 ":" $2 }' "$MAPPING_CSV"
)"

missing_keys=()
for key in "${required_keys[@]}"; do
  if ! grep -Fxq "$key" <<<"$exported_keys"; then
    missing_keys+=("$key")
  fi
done

if (( ${#missing_keys[@]} > 0 )); then
  printf '%s\n' "${missing_keys[@]}" >&2
  fail "required mappings could not be resolved automatically from legacy accounts"
fi

psql "$SOURCE_DSN" -v ON_ERROR_STOP=1 -v csv_file="$OUTLET_CSV" -v source_outlet_table="$SOURCE_OUTLET_TABLE" -v source_outlet_deleted_filter="$SOURCE_OUTLET_DELETED_FILTER" <<'SQL'
\copy (
  SELECT id AS outlet_id
  FROM :"source_outlet_table"
  WHERE 1=1
    :source_outlet_deleted_filter
  ORDER BY id ASC
) TO :'csv_file' WITH CSV HEADER
SQL

OUTLET_COUNT="$(tail -n +2 "$OUTLET_CSV" | wc -l | tr -d ' ')"
SEED_COUNT="$(tail -n +2 "$MAPPING_CSV" | wc -l | tr -d ' ')"

log "Resolved ${SEED_COUNT} mapping rules"
log "Loaded ${OUTLET_COUNT} active outlets from legacy source"

if [[ "$DRY_RUN" -eq 1 ]]; then
  log "Dry run enabled. Selected mapping candidates:"
  column -s, -t "$MAPPING_CSV" || cat "$MAPPING_CSV"
  exit 0
fi

log "Seeding tblaccount_mappings with mode=${MODE} scope=${SCOPE}"

psql "$TARGET_DSN" -v ON_ERROR_STOP=1 -v csv_mappings="$MAPPING_CSV" -v csv_outlets="$OUTLET_CSV" -v seed_mode="$MODE" -v seed_scope="$SCOPE" <<'SQL'
BEGIN;

CREATE TABLE IF NOT EXISTS public.tblaccount_mappings (
  id BIGSERIAL PRIMARY KEY,
  outlet_id BIGINT NOT NULL DEFAULT 0,
  account_id VARCHAR(10) NOT NULL,
  transaction_type VARCHAR(50) NOT NULL,
  purpose VARCHAR(50) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
);

UPDATE tblaccount_mappings
SET transaction_type = LOWER(BTRIM(COALESCE(transaction_type, ''))),
    purpose = LOWER(BTRIM(COALESCE(purpose, '')))
WHERE transaction_type IS DISTINCT FROM LOWER(BTRIM(COALESCE(transaction_type, '')))
   OR purpose IS DISTINCT FROM LOWER(BTRIM(COALESCE(purpose, '')));

DELETE FROM tblaccount_mappings
WHERE id IN (
  SELECT id
  FROM (
    SELECT
      id,
      ROW_NUMBER() OVER (
        PARTITION BY outlet_id, transaction_type, purpose
        ORDER BY updated_at DESC NULLS LAST, id DESC
      ) AS row_num
    FROM tblaccount_mappings
  ) ranked
  WHERE ranked.row_num > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tblaccount_mappings_scope
  ON tblaccount_mappings (outlet_id, transaction_type, purpose);

CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_account
  ON tblaccount_mappings (account_id);

CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_outlet_active
  ON tblaccount_mappings (outlet_id, is_active);

CREATE TEMP TABLE tmp_seed_mappings (
  transaction_type TEXT,
  purpose TEXT,
  account_id TEXT,
  priority INTEGER
);

CREATE TEMP TABLE tmp_seed_outlets (
  outlet_id BIGINT
);

\copy tmp_seed_mappings FROM :'csv_mappings' WITH CSV HEADER
\copy tmp_seed_outlets FROM :'csv_outlets' WITH CSV HEADER

DELETE FROM tmp_seed_mappings
WHERE NOT EXISTS (
  SELECT 1
  FROM tblaccounts accounts
  WHERE accounts.id = LEFT(BTRIM(COALESCE(tmp_seed_mappings.account_id, '')), 10)
);

WITH normalized_seed AS (
  SELECT
    LOWER(BTRIM(transaction_type)) AS transaction_type,
    LOWER(BTRIM(purpose)) AS purpose,
    LEFT(BTRIM(account_id), 10) AS account_id
  FROM tmp_seed_mappings
  WHERE BTRIM(COALESCE(account_id, '')) <> ''
),
global_upsert AS (
  INSERT INTO tblaccount_mappings (
    outlet_id,
    account_id,
    transaction_type,
    purpose,
    is_active,
    created_at,
    updated_at
  )
  SELECT
    0 AS outlet_id,
    account_id,
    transaction_type,
    purpose,
    TRUE,
    NOW(),
    NOW()
  FROM normalized_seed
  ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
    SET account_id = CASE
        WHEN :'seed_mode' = 'replace' THEN EXCLUDED.account_id
        ELSE tblaccount_mappings.account_id
      END,
        is_active = TRUE,
        updated_at = NOW()
  RETURNING 1
),
outlet_upsert AS (
  INSERT INTO tblaccount_mappings (
    outlet_id,
    account_id,
    transaction_type,
    purpose,
    is_active,
    created_at,
    updated_at
  )
  SELECT
    outlets.outlet_id,
    seed.account_id,
    seed.transaction_type,
    seed.purpose,
    TRUE,
    NOW(),
    NOW()
  FROM normalized_seed seed
  CROSS JOIN (
    SELECT DISTINCT outlet_id
    FROM tmp_seed_outlets
    WHERE COALESCE(outlet_id, 0) > 0
  ) outlets
  WHERE :'seed_scope' = 'all-outlets'
  ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
    SET account_id = CASE
        WHEN :'seed_mode' = 'replace' THEN EXCLUDED.account_id
        ELSE tblaccount_mappings.account_id
      END,
        is_active = TRUE,
        updated_at = NOW()
  RETURNING 1
)
SELECT
  (SELECT COUNT(*) FROM global_upsert) AS global_rows,
  (SELECT COUNT(*) FROM outlet_upsert) AS outlet_rows;

COMMIT;
SQL

GLOBAL_COUNT="$(psql "$TARGET_DSN" -At -c "SELECT COUNT(*) FROM tblaccount_mappings WHERE outlet_id = 0")"
if [[ "$SCOPE" == "all-outlets" ]]; then
  OUTLET_MAPPING_COUNT="$(psql "$TARGET_DSN" -At -c "SELECT COUNT(*) FROM tblaccount_mappings WHERE outlet_id <> 0")"
  log "Global mapping rows : ${GLOBAL_COUNT}"
  log "Outlet mapping rows : ${OUTLET_MAPPING_COUNT}"
else
  log "Global mapping rows : ${GLOBAL_COUNT}"
fi

log "Done."
