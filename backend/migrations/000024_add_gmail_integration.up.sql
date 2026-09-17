CREATE TABLE gmail_accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    email_address VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry TIMESTAMP NOT NULL,
    last_history_id VARCHAR(50) NOT NULL DEFAULT '',
    needs_reauth BOOLEAN NOT NULL DEFAULT false,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE emails (
    id SERIAL PRIMARY KEY,
    gmail_account_id INTEGER NOT NULL REFERENCES gmail_accounts(id) ON DELETE CASCADE,
    gmail_message_id VARCHAR(100) NOT NULL,
    thread_id VARCHAR(100) NOT NULL DEFAULT '',
    folder VARCHAR(10) NOT NULL,
    from_address VARCHAR(255) NOT NULL DEFAULT '',
    from_name VARCHAR(255) NOT NULL DEFAULT '',
    to_addresses VARCHAR(1000) NOT NULL DEFAULT '',
    subject VARCHAR(998) NOT NULL DEFAULT '',
    snippet VARCHAR(1000) NOT NULL DEFAULT '',
    has_attachments BOOLEAN NOT NULL DEFAULT false,
    attachment_meta JSONB NOT NULL DEFAULT '[]',
    is_read BOOLEAN NOT NULL DEFAULT false,
    received_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (gmail_account_id, gmail_message_id)
);

CREATE INDEX idx_emails_account_folder_received ON emails (gmail_account_id, folder, received_at DESC);

CREATE TABLE invoice_notices (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    period_start DATE,
    period_end DATE,
    gmail_message_id VARCHAR(100) NOT NULL,
    sent_by INTEGER NOT NULL REFERENCES users(id),
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoice_notices_project_period ON invoice_notices (project_id, period_start, period_end);

INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('gmail.manage', 'Manage Gmail Integration', 'Can connect the Gmail account and view/send synced emails', 'gmail', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'super_admin' AND p.name = 'gmail.manage';
