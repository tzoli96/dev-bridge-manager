CREATE TABLE client_status_emails (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sent_by INTEGER REFERENCES users(id),
    sent_at TIMESTAMP,
    gmail_message_id VARCHAR(255),
    dismissed_by INTEGER REFERENCES users(id),
    dismissed_at TIMESTAMP
);

CREATE INDEX idx_client_status_emails_client ON client_status_emails (client_id);
CREATE INDEX idx_client_status_emails_status ON client_status_emails (status);
