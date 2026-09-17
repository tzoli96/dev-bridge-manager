-- backend/migrations/000016_create_invoices_and_billingo_settings.up.sql

ALTER TABLE clients ADD COLUMN billingo_partner_id VARCHAR(100);

CREATE TABLE invoices (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    billingo_invoice_id VARCHAR(50),
    billingo_invoice_number VARCHAR(50),
    pricing_type VARCHAR(20) NOT NULL,
    period_start DATE,
    period_end DATE,
    amount NUMERIC(12,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'created',
    error_message TEXT,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_invoices_project_id ON invoices(project_id);
CREATE INDEX idx_invoices_client_id ON invoices(client_id);
CREATE INDEX idx_invoices_status ON invoices(status);

CREATE TABLE billingo_settings (
    id SERIAL PRIMARY KEY,
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    block_id VARCHAR(50) NOT NULL DEFAULT '',
    updated_by INTEGER REFERENCES users(id),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO billingo_settings (id, api_key, block_id) VALUES (1, '', '');

-- Add invoices / billingo_settings permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('invoices.create', 'Create Invoices', 'Can create invoices for a project', 'invoices', 'create'),
    ('invoices.read', 'View Invoices', 'Can view a project''s invoice history', 'invoices', 'read'),
    ('billingo_settings.manage', 'Manage Billingo Settings', 'Can view and update the Billingo API key/settings', 'billingo_settings', 'manage');

-- Super Admin and Admin get all three
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin') AND p.name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage');

-- Manager gets invoices.create and invoices.read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('invoices.create', 'invoices.read');

-- User gets invoices.read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name = 'invoices.read';
