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

CREATE UNIQUE INDEX IF NOT EXISTS idx_tblaccount_mappings_scope
  ON public.tblaccount_mappings (outlet_id, transaction_type, purpose);

CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_account
  ON public.tblaccount_mappings (account_id);

WITH outlet_ids AS (
  SELECT DISTINCT outlet_id
  FROM public.tbltransactions
  WHERE outlet_id IS NOT NULL
),
journal_base AS (
  SELECT
    je.outlet_id,
    LOWER(BTRIM(COALESCE(jl.description, ''))) AS description_key,
    jl.account_id
  FROM public.tbljournal_entries je
  JOIN public.tbljournal_lines jl ON jl.journal_entry_id = je.id
),
ledger_base AS (
  SELECT
    t.outlet_id,
    LOWER(BTRIM(COALESCE(t.payment_method, ''))) AS raw_payment_method,
    LOWER(BTRIM(COALESCE(jl.description, ''))) AS description_key,
    jl.account_id
  FROM public.tbltransactions t
  JOIN public.tbljournal_entries je ON je.id = t.journal_entry_id
  JOIN public.tbljournal_lines jl ON jl.journal_entry_id = je.id
  WHERE t.journal_entry_id IS NOT NULL
),
settlement_by_outlet AS (
  SELECT
    outlet_id,
    CASE
      WHEN raw_payment_method LIKE '%qris%' THEN 'qris'
      WHEN raw_payment_method LIKE '%transfer%' THEN 'transfer'
      WHEN raw_payment_method LIKE '%bayarnanti%' THEN 'bayarnanti'
      WHEN raw_payment_method LIKE '%cash%' OR raw_payment_method LIKE '%tunai%' OR raw_payment_method = '' THEN 'cash'
      ELSE raw_payment_method
    END AS purpose,
    account_id,
    COUNT(*) AS total_rows,
    ROW_NUMBER() OVER (
      PARTITION BY
        outlet_id,
        CASE
          WHEN raw_payment_method LIKE '%qris%' THEN 'qris'
          WHEN raw_payment_method LIKE '%transfer%' THEN 'transfer'
          WHEN raw_payment_method LIKE '%bayarnanti%' THEN 'bayarnanti'
          WHEN raw_payment_method LIKE '%cash%' OR raw_payment_method LIKE '%tunai%' OR raw_payment_method = '' THEN 'cash'
          ELSE raw_payment_method
        END
      ORDER BY COUNT(*) DESC, account_id ASC
    ) AS rn
  FROM ledger_base
  WHERE description_key = 'penerimaan penjualan'
  GROUP BY
    outlet_id,
    CASE
      WHEN raw_payment_method LIKE '%qris%' THEN 'qris'
      WHEN raw_payment_method LIKE '%transfer%' THEN 'transfer'
      WHEN raw_payment_method LIKE '%bayarnanti%' THEN 'bayarnanti'
      WHEN raw_payment_method LIKE '%cash%' OR raw_payment_method LIKE '%tunai%' OR raw_payment_method = '' THEN 'cash'
      ELSE raw_payment_method
    END,
    account_id
),
settlement_global AS (
  SELECT
    purpose,
    account_id,
    total_rows,
    ROW_NUMBER() OVER (
      PARTITION BY purpose
      ORDER BY total_rows DESC, account_id ASC
    ) AS rn
  FROM (
    SELECT
      purpose,
      account_id,
      SUM(total_rows) AS total_rows
    FROM settlement_by_outlet
    GROUP BY purpose, account_id
  ) x
),
sales_by_outlet AS (
  SELECT
    outlet_id,
    account_id,
    COUNT(*) AS total_rows,
    ROW_NUMBER() OVER (
      PARTITION BY outlet_id
      ORDER BY COUNT(*) DESC, account_id ASC
    ) AS rn
  FROM journal_base
  WHERE description_key = 'penjualan barang'
  GROUP BY outlet_id, account_id
),
sales_global AS (
  SELECT
    account_id,
    total_rows,
    ROW_NUMBER() OVER (ORDER BY total_rows DESC, account_id ASC) AS rn
  FROM (
    SELECT account_id, SUM(total_rows) AS total_rows
    FROM sales_by_outlet
    GROUP BY account_id
  ) x
),
discount_by_outlet AS (
  SELECT
    outlet_id,
    account_id,
    COUNT(*) AS total_rows,
    ROW_NUMBER() OVER (
      PARTITION BY outlet_id
      ORDER BY COUNT(*) DESC, account_id ASC
    ) AS rn
  FROM journal_base
  WHERE description_key = 'diskon penjualan'
  GROUP BY outlet_id, account_id
),
discount_global AS (
  SELECT
    account_id,
    total_rows,
    ROW_NUMBER() OVER (ORDER BY total_rows DESC, account_id ASC) AS rn
  FROM (
    SELECT account_id, SUM(total_rows) AS total_rows
    FROM discount_by_outlet
    GROUP BY account_id
  ) x
),
inventory_account AS (
  SELECT id AS account_id
  FROM public.tblaccounts
  WHERE LOWER(COALESCE(name, '')) LIKE '%persediaan%'
     OR LOWER(COALESCE(name, '')) LIKE '%inventory%'
  ORDER BY
    CASE WHEN LOWER(COALESCE(name, '')) = 'persediaan barang' THEN 0 ELSE 1 END,
    id
  LIMIT 1
),
seed_rows AS (
  SELECT
    0::bigint AS outlet_id,
    'sale'::varchar(50) AS transaction_type,
    purpose::varchar(50) AS purpose,
    account_id::varchar(10) AS account_id
  FROM settlement_global
  WHERE rn = 1

  UNION ALL

  SELECT
    outlet_id::bigint,
    'sale'::varchar(50),
    purpose::varchar(50),
    account_id::varchar(10)
  FROM settlement_by_outlet
  WHERE rn = 1

  UNION ALL

  SELECT
    0::bigint,
    'sale'::varchar(50),
    'sales'::varchar(50),
    account_id::varchar(10)
  FROM sales_global
  WHERE rn = 1

  UNION ALL

  SELECT
    outlet_id::bigint,
    'sale'::varchar(50),
    'sales'::varchar(50),
    account_id::varchar(10)
  FROM sales_by_outlet
  WHERE rn = 1

  UNION ALL

  SELECT
    0::bigint,
    'sale'::varchar(50),
    'discount'::varchar(50),
    account_id::varchar(10)
  FROM discount_global
  WHERE rn = 1

  UNION ALL

  SELECT
    outlet_id::bigint,
    'sale'::varchar(50),
    'discount'::varchar(50),
    account_id::varchar(10)
  FROM discount_by_outlet
  WHERE rn = 1

  UNION ALL

  SELECT
    0::bigint,
    'purchase'::varchar(50),
    'inventory'::varchar(50),
    account_id::varchar(10)
  FROM inventory_account

  UNION ALL

  SELECT
    o.outlet_id::bigint,
    'purchase'::varchar(50),
    'inventory'::varchar(50),
    i.account_id::varchar(10)
  FROM outlet_ids o
  CROSS JOIN inventory_account i
),
deduped_seed AS (
  SELECT DISTINCT outlet_id, transaction_type, purpose, account_id
  FROM seed_rows
)
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
  outlet_id,
  account_id,
  transaction_type,
  purpose,
  TRUE,
  NOW(),
  NOW()
FROM deduped_seed
ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
SET
  account_id = EXCLUDED.account_id,
  is_active = TRUE,
  updated_at = NOW();

COMMIT;
