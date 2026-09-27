ALTER TABLE invoices ADD COLUMN invoiced_hours DOUBLE PRECISION;

CREATE TABLE invoice_reconciliation_flags (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    invoice_id INTEGER REFERENCES invoices(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL,
    details TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    dismissed_by INTEGER REFERENCES users(id),
    dismissed_at TIMESTAMP
);

CREATE INDEX idx_invoice_reconciliation_flags_project ON invoice_reconciliation_flags (project_id);
CREATE INDEX idx_invoice_reconciliation_flags_status ON invoice_reconciliation_flags (status);
