ALTER TABLE projects
    ADD COLUMN auto_invoice_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN auto_invoice_client_id INTEGER REFERENCES clients(id);
