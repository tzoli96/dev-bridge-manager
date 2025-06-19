-- 000006_create_projects_table.up.sql
CREATE TABLE projects (
                          id SERIAL PRIMARY KEY,
                          name VARCHAR(255) NOT NULL,
                          description TEXT,
                          status VARCHAR(50) DEFAULT 'active',
                          created_by INTEGER NOT NULL REFERENCES users(id),
                          created_at TIMESTAMP DEFAULT NOW(),
                          updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_projects_created_by ON projects(created_by);
CREATE INDEX idx_projects_status ON projects(status);

-- Insert test data
INSERT INTO projects (name, description, created_by) VALUES
                                                         ('E-commerce Platform', 'Modern e-commerce solution with React and Node.js', 1),
                                                         ('Mobile App Development', 'Cross-platform mobile application using React Native', 2),
                                                         ('Data Analytics Dashboard', 'Real-time analytics dashboard for business intelligence', 1),
                                                         ('API Gateway Service', 'Microservices API gateway implementation', 1),
                                                         ('DevOps Automation', 'CI/CD pipeline automation and infrastructure as code', 1);

-- Add project permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
                                                                                ('projects.create', 'Create Projects', 'Can create new projects', 'projects', 'create'),
                                                                                ('projects.read', 'View Projects', 'Can view project information', 'projects', 'read'),
                                                                                ('projects.update', 'Update Projects', 'Can update project information', 'projects', 'update'),
                                                                                ('projects.delete', 'Delete Projects', 'Can delete projects', 'projects', 'delete'),
                                                                                ('projects.list', 'List Projects', 'Can list all projects', 'projects', 'list');

-- Assign project permissions to roles
-- Super Admin gets all project permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'super_admin' AND p.name LIKE 'projects.%';

-- Admin gets all project permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name LIKE 'projects.%';

-- Manager gets read and list project permissions only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('projects.read', 'projects.list');

-- User gets only read and list project permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name IN ('projects.read', 'projects.list');

-- 000006_create_projects_table.down.sql
DROP TABLE IF EXISTS projects;