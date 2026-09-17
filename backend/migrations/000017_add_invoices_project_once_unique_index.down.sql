-- backend/migrations/000017_add_invoices_project_once_unique_index.down.sql

DROP INDEX IF EXISTS idx_invoices_fixed_price_created_once;
