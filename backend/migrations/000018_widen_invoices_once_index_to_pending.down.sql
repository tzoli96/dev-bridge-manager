-- backend/migrations/000018_widen_invoices_once_index_to_pending.down.sql

DROP INDEX IF EXISTS idx_invoices_fixed_price_once;
CREATE UNIQUE INDEX idx_invoices_fixed_price_created_once ON invoices(project_id) WHERE pricing_type = 'fixed' AND status = 'created';
