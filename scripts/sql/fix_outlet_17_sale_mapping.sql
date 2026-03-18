-- Fix outlet 17 sale revenue mapping.
-- Current bad mapping observed:
--   outlet_id = 17
--   transaction_type = 'sale'
--   purpose = 'sales'
--   account_id = '101'   -- Asset / Cash
--
-- Expected mapping:
--   account_id = '402'   -- POS Revenue

BEGIN;

UPDATE tblaccount_mappings
SET account_id = '402'
WHERE outlet_id = 17
  AND transaction_type = 'sale'
  AND purpose = 'sales'
  AND account_id = '101';

COMMIT;

-- Validation query:
-- SELECT id, outlet_id, transaction_type, purpose, account_id
-- FROM tblaccount_mappings
-- WHERE outlet_id = 17 AND transaction_type = 'sale'
-- ORDER BY purpose, id;
