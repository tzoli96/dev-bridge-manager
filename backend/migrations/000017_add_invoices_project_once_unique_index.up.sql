-- backend/migrations/000017_add_invoices_project_once_unique_index.up.sql

CREATE UNIQUE INDEX idx_invoices_fixed_price_created_once ON invoices(project_id) WHERE status = 'created' AND pricing_type = 'fixed';
