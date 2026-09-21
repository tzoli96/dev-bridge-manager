CREATE TABLE invoice_ready_emails (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    sent_by INTEGER NOT NULL REFERENCES users(id),
    sent_at TIMESTAMP NOT NULL DEFAULT NOW(),
    gmail_message_id VARCHAR(100) NOT NULL
);

CREATE INDEX idx_invoice_ready_emails_invoice ON invoice_ready_emails (invoice_id);
