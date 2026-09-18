DROP INDEX IF EXISTS idx_invoice_notices_status;

ALTER TABLE invoice_notices
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS invoice_id,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at;
