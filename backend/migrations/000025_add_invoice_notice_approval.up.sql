ALTER TABLE invoice_notices
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN invoice_id INTEGER REFERENCES invoices(id) ON DELETE SET NULL,
    ADD COLUMN approved_by INTEGER REFERENCES users(id),
    ADD COLUMN approved_at TIMESTAMP;

CREATE INDEX idx_invoice_notices_status ON invoice_notices (status);
