CREATE TABLE invoice_reminders (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    days_overdue INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sent_by INTEGER REFERENCES users(id),
    sent_at TIMESTAMP,
    gmail_message_id VARCHAR(100)
);

CREATE INDEX idx_invoice_reminders_invoice ON invoice_reminders (invoice_id);
CREATE INDEX idx_invoice_reminders_status ON invoice_reminders (status);
