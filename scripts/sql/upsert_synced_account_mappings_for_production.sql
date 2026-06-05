BEGIN;

-- Normalize legacy primary mapping tokens before we seed canonical values.
UPDATE public.tblaccounts
SET
  transaction_type = NULLIF(LOWER(BTRIM(COALESCE(transaction_type, ''))), ''),
  purpose = NULLIF(LOWER(BTRIM(COALESCE(purpose, ''))), '')
WHERE transaction_type IS NOT NULL
   OR purpose IS NOT NULL;

-- Clear legacy primary mappings that often recreate noisy rows in tblaccount_mappings.
UPDATE public.tblaccounts
SET
  transaction_type = NULL,
  purpose = NULL
WHERE id IN (
  '501',
  '1100',
  '1103',
  '1104',
  '1105',
  '1106',
  '4100',
  '4101',
  '4102',
  '4105',
  '5100',
  '6180'
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
VALUES
  ('101', 'Cash', 'Asset', TRUE, 0, 'sale', 'cash'),
  ('102', 'QRIS', 'Asset', TRUE, 0, 'sale', 'qris'),
  ('103', 'Bank Transfer', 'Asset', TRUE, 0, 'sale', 'transfer'),
  ('1101', 'Persediaan Barang', 'Asset', TRUE, 0, 'purchase', 'inventory'),
  ('2101', 'Hutang Usaha', 'Liability', TRUE, 0, 'purchase', 'credit'),
  ('2102', 'Pajak Penjualan', 'Liability', TRUE, 0, 'sale', 'tax'),
  ('2103', 'Saldo Outlet / Fee Payable', 'Liability', TRUE, 0, 'fee', 'balance'),
  ('402', 'POS Revenue', 'Revenue', TRUE, 0, 'sale', 'sales'),
  ('502', 'Sales Discount', 'Expense', TRUE, 0, 'sale', 'discount'),
  ('601', 'Harga Pokok Penjualan', 'Expense', TRUE, 0, 'sale', 'cogs'),
  ('602', 'Biaya Fee Outlet / MDR', 'Expense', TRUE, 0, 'fee', 'expense')
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  is_active = TRUE,
  outlet_id = 0,
  transaction_type = EXCLUDED.transaction_type,
  purpose = EXCLUDED.purpose;

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

UPDATE public.tblaccount_mappings
SET
  transaction_type = LOWER(BTRIM(COALESCE(transaction_type, ''))),
  purpose = LOWER(BTRIM(COALESCE(purpose, '')))
WHERE transaction_type IS DISTINCT FROM LOWER(BTRIM(COALESCE(transaction_type, '')))
   OR purpose IS DISTINCT FROM LOWER(BTRIM(COALESCE(purpose, '')));

DELETE FROM public.tblaccount_mappings
WHERE COALESCE(BTRIM(transaction_type), '') = ''
   OR COALESCE(BTRIM(purpose), '') = '';

DELETE FROM public.tblaccount_mappings
WHERE id IN (
  SELECT id
  FROM (
    SELECT
      id,
      ROW_NUMBER() OVER (
        PARTITION BY outlet_id, transaction_type, purpose
        ORDER BY updated_at DESC NULLS LAST, created_at DESC NULLS LAST, id DESC
      ) AS rn
    FROM public.tblaccount_mappings
  ) ranked
  WHERE rn > 1
);

WITH target_outlets AS (
  SELECT 0::bigint AS outlet_id
  UNION
  SELECT DISTINCT outlet_id::bigint
  FROM public.tbltransactions
  WHERE outlet_id IS NOT NULL
  UNION
  SELECT DISTINCT outlet_id::bigint
  FROM public.tbloutletfee
  WHERE outlet_id IS NOT NULL
),
seed AS (
  SELECT *
  FROM (
    VALUES
      ('sale', 'sales', '402'),
      ('sale', 'cash', '101'),
      ('sale', 'qris', '102'),
      ('sale', 'transfer', '103'),
      ('sale', 'tax', '2102'),
      ('sale', 'discount', '502'),
      ('sale', 'cogs', '601'),
      ('purchase', 'inventory', '1101'),
      ('purchase', 'cash', '101'),
      ('purchase', 'qris', '102'),
      ('purchase', 'transfer', '103'),
      ('purchase', 'credit', '2101'),
      ('fee', 'expense', '602'),
      ('fee', 'outlet_fee', '602'),
      ('sale', 'outlet_fee', '602'),
      ('sale', 'fee_expense', '602'),
      ('fee', 'balance', '2103'),
      ('fee', 'outlet_balance', '2103'),
      ('balance', 'outlet', '2103'),
      ('balance', 'liability', '2103'),
      ('deposit', 'balance', '2103')
  ) AS v(transaction_type, purpose, account_id)
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
  o.outlet_id,
  s.account_id,
  s.transaction_type,
  s.purpose,
  TRUE,
  NOW(),
  NOW()
FROM target_outlets o
CROSS JOIN seed s
ON CONFLICT (outlet_id, transaction_type, purpose) DO UPDATE
SET
  account_id = EXCLUDED.account_id,
  is_active = TRUE,
  updated_at = NOW();

COMMIT;
