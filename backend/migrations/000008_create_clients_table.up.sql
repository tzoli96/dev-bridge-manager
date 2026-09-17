-- 000008_create_clients_table.up.sql
CREATE TABLE clients (
                          id SERIAL PRIMARY KEY,
                          type VARCHAR(20) NOT NULL DEFAULT 'company', -- 'company' | 'individual'
                          name VARCHAR(255) NOT NULL,
                          tax_number VARCHAR(20),          -- adószám, kötelező cégnél
                          eu_vat_number VARCHAR(30),        -- közösségi adószám
                          company_reg_number VARCHAR(30),   -- cégjegyzékszám
                          billing_zip VARCHAR(10),
                          billing_city VARCHAR(150),
                          billing_address VARCHAR(255),
                          bank_account_number VARCHAR(50),
                          email VARCHAR(255),
                          phone VARCHAR(50),
                          notes TEXT,
                          is_active BOOLEAN DEFAULT TRUE,
                          created_by INTEGER NOT NULL REFERENCES users(id),
                          created_at TIMESTAMP DEFAULT NOW(),
                          updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_clients_created_by ON clients(created_by);
CREATE INDEX idx_clients_is_active ON clients(is_active);
CREATE INDEX idx_clients_name ON clients(name);

-- Add client permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
                                                                                 ('clients.create', 'Create Clients', 'Can create new clients', 'clients', 'create'),
                                                                                 ('clients.read', 'View Clients', 'Can view client information', 'clients', 'read'),
                                                                                 ('clients.update', 'Update Clients', 'Can update client information', 'clients', 'update'),
                                                                                 ('clients.delete', 'Delete Clients', 'Can delete clients', 'clients', 'delete'),
                                                                                 ('clients.list', 'List Clients', 'Can list all clients', 'clients', 'list');

-- Assign client permissions to roles
-- Super Admin and Admin get all client permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin') AND p.name LIKE 'clients.%';

-- Manager gets read and list only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('clients.read', 'clients.list');

-- User gets read and list only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name IN ('clients.read', 'clients.list');
