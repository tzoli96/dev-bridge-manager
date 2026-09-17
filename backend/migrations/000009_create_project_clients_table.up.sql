-- 000009_create_project_clients_table.up.sql
-- Projekt-ügyfél hozzárendelések (egy projekthez több ügyfél is tartozhat)
CREATE TABLE project_clients (
                                  id SERIAL PRIMARY KEY,
                                  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
                                  client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
                                  assigned_at TIMESTAMP DEFAULT NOW(),
                                  assigned_by INTEGER REFERENCES users(id),
                                  UNIQUE(project_id, client_id)
);

-- Indexes
CREATE INDEX idx_project_clients_project ON project_clients(project_id);
CREATE INDEX idx_project_clients_client ON project_clients(client_id);

-- Add project_clients permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
                                                                                 ('project_clients.create', 'Assign Clients to Projects', 'Can assign clients to projects', 'project_clients', 'create'),
                                                                                 ('project_clients.read', 'View Project Clients', 'Can view project-client links', 'project_clients', 'read'),
                                                                                 ('project_clients.delete', 'Remove Project Clients', 'Can remove clients from projects', 'project_clients', 'delete');

-- Super Admin and Admin get all project_clients permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin') AND p.name LIKE 'project_clients.%';

-- Manager can read and assign
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('project_clients.read', 'project_clients.create');

-- User can read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name = 'project_clients.read';
