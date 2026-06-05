BEGIN;

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
  ('2102', 'Pajak Penjualan', 'Liability', TRUE, 0, NULL, NULL),
  ('2103', 'Saldo Outlet / Fee Payable', 'Liability', TRUE, 0, NULL, NULL),
  ('5100', 'Harga Pokok Penjualan', 'Beban', TRUE, 0, NULL, NULL),
  ('6180', 'Biaya Fee Outlet / MDR', 'Beban', TRUE, 0, NULL, NULL)
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  is_active = TRUE,
  outlet_id = 0,
  transaction_type = NULL,
  purpose = NULL;

WITH target_outlets AS (
  SELECT 0::bigint AS outlet_id
  UNION
  SELECT DISTINCT outlet_id::bigint
  FROM public.tbltransactions
  WHERE outlet_id IS NOT NULL
),
seed AS (
  SELECT *
  FROM (
    VALUES
      ('sale', 'tax', '2102'),
      ('sale', 'cogs', '5100'),
      ('fee', 'expense', '6180'),
      ('fee', 'outlet_fee', '6180'),
      ('sale', 'outlet_fee', '6180'),
      ('sale', 'fee_expense', '6180'),
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
