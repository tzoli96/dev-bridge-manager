CREATE TABLE profit_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    minutes_per_inbound_email NUMERIC(6,2) NOT NULL DEFAULT 5,
    minutes_per_outbound_email NUMERIC(6,2) NOT NULL DEFAULT 10,
    default_capacity_hours_per_month NUMERIC(6,2) NOT NULL DEFAULT 120,
    underpriced_ratio_threshold NUMERIC(4,2) NOT NULL DEFAULT 0.60,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO profit_settings (id) VALUES (1);

CREATE TABLE client_meeting_allowances (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL UNIQUE REFERENCES clients(id) ON DELETE CASCADE,
    hours_per_month NUMERIC(6,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('profitability.read', 'View Profitability', 'Can view client/project profitability', 'profitability', 'read'),
    ('profitability.manage', 'Manage Profitability', 'Can edit profitability settings and meeting allowances', 'profitability', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin')
  AND p.name IN ('profitability.read', 'profitability.manage');
