#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POSSYSTEM_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_SOURCE_ENV="${POSSYSTEM_ROOT}/../apifitness/.env"
DEFAULT_TARGET_ENV="${POSSYSTEM_ROOT}/.env"

SOURCE_ENV_FILE="${SOURCE_ENV_FILE:-${DEFAULT_SOURCE_ENV}}"
TARGET_ENV_FILE="${TARGET_ENV_FILE:-${DEFAULT_TARGET_ENV}}"
SOURCE_DSN="${SOURCE_DSN:-}"
TARGET_DSN="${TARGET_DSN:-}"
SOURCE_ACCOUNT_TABLE="${SOURCE_ACCOUNT_TABLE:-}"
SOURCE_DELETED_FILTER="${SOURCE_DELETED_FILTER:-}"
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage:
  bash scripts/sync_legacy_accounting_accounts.sh [options]

Options:
  --source-env PATH   Path .env source legacy database. Default: ../apifitness/.env
  --target-env PATH   Path .env target POS database. Default: ./.env
  --source-dsn DSN    Override source PostgreSQL DSN.
  --target-dsn DSN    Override target PostgreSQL DSN.
  --source-table NAME Override source accounts table (tbaccounts or tblaccounts).
  --dry-run           Export and validate only, do not write to target DB.
  --help              Show this help.

Notes:
  - Source expected table: tbaccounts
  - Target expected table: tblaccounts
  - Script syncs chart-of-accounts master only.
  - Outlet-specific account mappings are intentionally not generated automatically.
EOF
}

log() {
  printf '[sync-accounts] %s\n' "$*"
}

fail() {
  printf '[sync-accounts] ERROR: %s\n' "$*" >&2
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

ensure_target_account_table() {
  local dsn="$1"

  psql "$dsn" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS public.tblaccounts (
  id character varying(10) NOT NULL,
  name character varying(100) NOT NULL,
  category character varying(50),
  is_active boolean DEFAULT true,
  outlet_id bigint,
  transaction_type character varying(50),
  purpose character varying(50),
  CONSTRAINT tblaccounts_pkey PRIMARY KEY (id)
);
SQL
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
    printf 'WHERE deleted_at IS NULL'
  else
    printf 'WHERE 1=1'
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

if [[ -z "$SOURCE_DSN" ]]; then
  SOURCE_DSN="$(read_env_var "$SOURCE_ENV_FILE" DATABASE_DSN)"
fi

if [[ -z "$TARGET_DSN" ]]; then
  TARGET_DSN="$(read_env_var "$TARGET_ENV_FILE" DATABASE_DSN)"
fi

[[ -n "$SOURCE_DSN" ]] || fail "source DSN is empty"
[[ -n "$TARGET_DSN" ]] || fail "target DSN is empty"

ensure_target_account_table "$TARGET_DSN"
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

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
CSV_FILE="${TMP_DIR}/legacy_accounts.csv"

log "Source env : $SOURCE_ENV_FILE"
log "Target env : $TARGET_ENV_FILE"
log "Source table : $SOURCE_ACCOUNT_TABLE"
log "Exporting legacy accounts from ${SOURCE_ACCOUNT_TABLE}"

psql "$SOURCE_DSN" -v ON_ERROR_STOP=1 -v csv_file="$CSV_FILE" -v source_account_table="$SOURCE_ACCOUNT_TABLE" -v source_deleted_filter="$SOURCE_DELETED_FILTER" <<'SQL'
\copy (
  SELECT DISTINCT ON (normalized_id)
    normalized_id AS id,
    COALESCE(NULLIF(BTRIM(account_name), ''), normalized_id) AS name,
    normalized_type AS category,
    TRUE AS is_active,
    0 AS outlet_id,
    '' AS transaction_type,
    '' AS purpose
  FROM (
    SELECT
      BTRIM(COALESCE(account_id::text, '')) AS normalized_id,
      account_name,
      CASE LOWER(BTRIM(COALESCE(account_type, '')))
        WHEN 'asset' THEN 'Asset'
        WHEN 'liability' THEN 'Liability'
        WHEN 'equity' THEN 'Equity'
        WHEN 'revenue' THEN 'Revenue'
        WHEN 'expense' THEN 'Expense'
        ELSE COALESCE(NULLIF(BTRIM(account_type), ''), 'Uncategorized')
      END AS normalized_type,
      updated_at,
      created_at,
      id
    FROM :"source_account_table"
    :source_deleted_filter
  ) source_accounts
  WHERE normalized_id <> ''
  ORDER BY normalized_id, updated_at DESC NULLS LAST, created_at DESC NULLS LAST, id DESC
) TO :'csv_file' WITH CSV HEADER
SQL

[[ -s "$CSV_FILE" ]] || fail "legacy account export is empty"

SOURCE_COUNT="$(tail -n +2 "$CSV_FILE" | wc -l | tr -d ' ')"
log "Exported ${SOURCE_COUNT} account rows"

if [[ "$DRY_RUN" -eq 1 ]]; then
  log "Dry run enabled. Sample exported rows:"
  head -n 6 "$CSV_FILE"
  exit 0
fi

log "Upserting accounts into tblaccounts"

psql "$TARGET_DSN" -v ON_ERROR_STOP=1 -v csv_file="$CSV_FILE" <<'SQL'
BEGIN;

CREATE TEMP TABLE tmp_legacy_accounts (
  id TEXT,
  name TEXT,
  category TEXT,
  is_active BOOLEAN,
  outlet_id BIGINT,
  transaction_type TEXT,
  purpose TEXT
);

\copy tmp_legacy_accounts FROM :'csv_file' WITH CSV HEADER

WITH normalized AS (
  SELECT
    LEFT(BTRIM(COALESCE(id, '')), 10) AS id,
    LEFT(COALESCE(NULLIF(BTRIM(name), ''), BTRIM(COALESCE(id, ''))), 100) AS name,
    LEFT(COALESCE(NULLIF(BTRIM(category), ''), 'Uncategorized'), 50) AS category,
    COALESCE(is_active, TRUE) AS is_active,
    COALESCE(outlet_id, 0) AS outlet_id,
    NULLIF(LOWER(BTRIM(COALESCE(transaction_type, ''))), '') AS transaction_type,
    NULLIF(LOWER(BTRIM(COALESCE(purpose, ''))), '') AS purpose
  FROM tmp_legacy_accounts
  WHERE BTRIM(COALESCE(id, '')) <> ''
),
upserted AS (
  INSERT INTO tblaccounts (id, name, category, is_active, outlet_id, transaction_type, purpose)
  SELECT
    id,
    name,
    category,
    is_active,
    outlet_id,
    transaction_type,
    purpose
  FROM normalized
  ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name,
        category = EXCLUDED.category,
        is_active = EXCLUDED.is_active,
        outlet_id = CASE
          WHEN COALESCE(tblaccounts.outlet_id, 0) = 0 THEN EXCLUDED.outlet_id
          ELSE tblaccounts.outlet_id
        END,
        transaction_type = COALESCE(NULLIF(tblaccounts.transaction_type, ''), EXCLUDED.transaction_type),
        purpose = COALESCE(NULLIF(tblaccounts.purpose, ''), EXCLUDED.purpose)
  RETURNING id
)
SELECT COUNT(*) AS synced_rows FROM upserted;

COMMIT;
SQL

TARGET_COUNT="$(psql "$TARGET_DSN" -At -c "SELECT COUNT(*) FROM tblaccounts")"

log "Sync complete. Target tblaccounts rows: ${TARGET_COUNT}"
log "Done."
