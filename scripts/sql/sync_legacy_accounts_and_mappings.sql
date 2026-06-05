-- Run this SQL while connected to the POS database.
-- It pulls legacy chart-of-accounts and outlet list from the API Fitness database
-- via dblink, then syncs:
--   1. public.tblaccounts
--   2. public.tblaccount_mappings
--
-- Replace the dblink connection string below before running.
--
-- Recommended:
-- BEGIN;
-- \i scripts/sql/sync_legacy_accounts_and_mappings.sql
-- COMMIT;

CREATE EXTENSION IF NOT EXISTS dblink;

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

CREATE UNIQUE INDEX IF NOT EXISTS idx_tblaccount_mappings_scope
    ON public.tblaccount_mappings (outlet_id, transaction_type, purpose);

CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_account
    ON public.tblaccount_mappings (account_id);

CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_outlet_active
    ON public.tblaccount_mappings (outlet_id, is_active);

DROP TABLE IF EXISTS tmp_legacy_accounts;
CREATE TEMP TABLE tmp_legacy_accounts AS
SELECT *
FROM dblink(
    'host=127.0.0.1 port=5432 dbname=apifitness user=adminbaru password=REPLACE_ME',
    $DBLINK$
    SELECT DISTINCT ON (BTRIM(COALESCE(account_id::text, '')))
        LEFT(BTRIM(COALESCE(account_id::text, '')), 10) AS id,
        LEFT(COALESCE(NULLIF(BTRIM(account_name), ''), BTRIM(COALESCE(account_id::text, ''))), 100) AS name,
        LEFT(
            CASE LOWER(BTRIM(COALESCE(account_type, '')))
                WHEN 'asset' THEN 'Asset'
                WHEN 'liability' THEN 'Liability'
                WHEN 'equity' THEN 'Equity'
                WHEN 'revenue' THEN 'Revenue'
                WHEN 'expense' THEN 'Expense'
                ELSE COALESCE(NULLIF(BTRIM(account_type), ''), 'Uncategorized')
            END,
            50
        ) AS category
    FROM public.tbaccounts
    WHERE deleted_at IS NULL
      AND BTRIM(COALESCE(account_id::text, '')) <> ''
    ORDER BY
        BTRIM(COALESCE(account_id::text, '')),
        updated_at DESC NULLS LAST,
        created_at DESC NULLS LAST,
        id DESC
    $DBLINK$
) AS legacy_accounts (
    id TEXT,
    name TEXT,
    category TEXT
);

INSERT INTO public.tblaccounts (
    id,
    name,
    category,
    is_active,
    outlet_id,
    transaction_type,
    purpose
)
SELECT
    id,
    name,
    category,
    TRUE,
    0,
    NULL,
    NULL
FROM tmp_legacy_accounts
ON CONFLICT (id) DO UPDATE
SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    is_active = TRUE,
    outlet_id = CASE
        WHEN COALESCE(public.tblaccounts.outlet_id, 0) = 0 THEN EXCLUDED.outlet_id
        ELSE public.tblaccounts.outlet_id
    END,
    transaction_type = COALESCE(NULLIF(public.tblaccounts.transaction_type, ''), EXCLUDED.transaction_type),
    purpose = COALESCE(NULLIF(public.tblaccounts.purpose, ''), EXCLUDED.purpose);

DROP TABLE IF EXISTS tmp_legacy_outlets;
CREATE TEMP TABLE tmp_legacy_outlets AS
SELECT *
FROM dblink(
    'host=127.0.0.1 port=5432 dbname=apifitness user=adminbaru password=REPLACE_ME',
    $DBLINK$
    SELECT id
    FROM public.tboutlet
    WHERE deleted_at IS NULL
    ORDER BY id
    $DBLINK$
) AS legacy_outlets (
    outlet_id BIGINT
);

DROP TABLE IF EXISTS tmp_seed_mappings;
CREATE TEMP TABLE tmp_seed_mappings AS
WITH normalized_accounts AS (
    SELECT
        LOWER(BTRIM(id)) AS account_id,
        LOWER(BTRIM(name)) AS account_name,
        LOWER(BTRIM(category)) AS account_category
    FROM public.tblaccounts
    WHERE COALESCE(is_active, TRUE) = TRUE
      AND BTRIM(COALESCE(id, '')) <> ''
),
candidate_rules AS (
    SELECT
        'sale'::text AS transaction_type,
        'cash'::text AS purpose,
        account_id,
        CASE
            WHEN account_category = 'asset' AND account_name ~ '(petty cash|cash on hand|kas kecil)' THEN 5
            WHEN account_category = 'asset' AND account_name ~ '(^|[^a-z])(cash|kas)($|[^a-z])' THEN 10
            WHEN account_category = 'asset' AND account_name LIKE '%cash%' THEN 20
            WHEN account_category = 'asset' AND account_name LIKE '%kas%' THEN 30
            ELSE 999
        END AS priority
    FROM normalized_accounts

    UNION ALL

    SELECT
        'sale',
        'qris',
        account_id,
        CASE
            WHEN account_category = 'asset' AND account_name ~ '(^|[^a-z])qris($|[^a-z])' THEN 5
            WHEN account_category = 'asset' AND account_name LIKE '%qris%' THEN 10
            ELSE 999
        END
    FROM normalized_accounts

    UNION ALL

    SELECT
        'sale',
        'transfer',
        account_id,
        CASE
            WHEN account_category = 'asset' AND account_name ~ '(bank transfer|transfer bank)' THEN 5
            WHEN account_category = 'asset' AND account_name ~ '(^|[^a-z])transfer($|[^a-z])' THEN 10
            WHEN account_category = 'asset' AND account_name ~ '(bank|bca|bni|bri|mandiri|permata|cimb|danamon|rekening|tabungan)' THEN 20
            ELSE 999
        END
    FROM normalized_accounts

    UNION ALL

    SELECT
        'sale',
        'sales',
        account_id,
        CASE
            WHEN account_category = 'revenue' AND account_name ~ '(pos revenue|pendapatan pos|penjualan pos)' THEN 5
            WHEN account_category = 'revenue' AND account_name ~ '(penjualan|sales|revenue|pendapatan)' THEN 10
            WHEN account_category = 'revenue' THEN 30
            ELSE 999
        END
    FROM normalized_accounts
    WHERE account_name !~ '(discount|diskon|retur|return|refund|tax|pajak|ppn)'

    UNION ALL

    SELECT
        'sale',
        'tax',
        account_id,
        CASE
            WHEN account_category = 'liability' AND account_name ~ '(ppn keluaran|pajak keluaran|sales tax)' THEN 5
            WHEN account_category = 'liability' AND account_name ~ '(tax|pajak|ppn|vat)' THEN 10
            WHEN account_category = 'liability' THEN 40
            ELSE 999
        END
    FROM normalized_accounts

    UNION ALL

    SELECT
        'sale',
        'cogs',
        account_id,
        CASE
            WHEN account_category = 'expense' AND account_name ~ '(harga pokok penjualan|beban pokok penjualan)' THEN 5
            WHEN account_category = 'expense' AND account_name ~ '(^|[^a-z])hpp($|[^a-z])' THEN 10
            WHEN account_category = 'expense' AND account_name ~ '(cogs|cost of goods|harga pokok|beban pokok)' THEN 15
            ELSE 999
        END
    FROM normalized_accounts

    UNION ALL

    SELECT
        'purchase',
        'inventory',
        account_id,
        CASE
            WHEN account_category = 'asset' AND account_name ~ '(persediaan barang|inventory asset)' THEN 5
            WHEN account_category = 'asset' AND account_name ~ '(persediaan|inventory|stock|stok)' THEN 10
            ELSE 999
        END
    FROM normalized_accounts
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
    account_id
FROM ranked
WHERE row_num = 1;

-- Preview result before apply if needed:
-- SELECT * FROM tmp_seed_mappings ORDER BY transaction_type, purpose;

-- Global fallback mapping (outlet_id = 0)
INSERT INTO public.tblaccount_mappings (
    outlet_id,
    account_id,
    transaction_type,
    purpose,
    is_active,
    created_at,
    updated_at
)
SELECT
    0,
    account_id,
    transaction_type,
    purpose,
    TRUE,
    NOW(),
    NOW()
FROM tmp_seed_mappings
ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
SET
    is_active = TRUE,
    updated_at = NOW()
WHERE public.tblaccount_mappings.account_id IS NULL
   OR public.tblaccount_mappings.account_id = '';

-- Outlet-by-outlet mapping
INSERT INTO public.tblaccount_mappings (
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
FROM tmp_seed_mappings seed
CROSS JOIN tmp_legacy_outlets outlets
WHERE COALESCE(outlets.outlet_id, 0) > 0
ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
SET
    is_active = TRUE,
    updated_at = NOW()
WHERE public.tblaccount_mappings.account_id IS NULL
   OR public.tblaccount_mappings.account_id = '';

-- Validation queries:
-- SELECT * FROM public.tblaccounts ORDER BY id;
-- SELECT outlet_id, transaction_type, purpose, account_id
-- FROM public.tblaccount_mappings
-- WHERE transaction_type IN ('sale', 'purchase')
-- ORDER BY outlet_id, transaction_type, purpose;
