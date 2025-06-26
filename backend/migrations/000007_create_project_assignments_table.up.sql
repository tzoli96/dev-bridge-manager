-- 000007_create_project_assignments_table.up.sql
-- Projekt-felhasználó hozzárendelések
CREATE TABLE project_assignments (
                                     id SERIAL PRIMARY KEY,
                                     project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
                                     user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                     role VARCHAR(100) DEFAULT 'member', -- 'owner', 'manager', 'member', 'viewer'
                                     assigned_at TIMESTAMP DEFAULT NOW(),
                                     assigned_by INTEGER REFERENCES users(id),
                                     is_active BOOLEAN DEFAULT TRUE,
                                     UNIQUE(project_id, user_id) -- Egy user csak egyszer lehet hozzárendelve egy projekthez
);

-- Indexek a gyors keresésért
CREATE INDEX idx_project_assignments_project ON project_assignments(project_id);
CREATE INDEX idx_project_assignments_user ON project_assignments(user_id);
CREATE INDEX idx_project_assignments_active ON project_assignments(is_active);

-- Test adatok beszúrása
INSERT INTO project_assignments (project_id, user_id, role, assigned_by) VALUES
                                                                             (1, 1, 'owner', 1),    -- John Doe tulajdonosa az E-commerce platformnak
                                                                             (1, 3, 'member', 1),   -- Bob Wilson tag
                                                                             (1, 4, 'member', 1),   -- Alice Johnson tag
                                                                             (2, 2, 'owner', 2),    -- Jane Smith tulajdonosa a Mobile App-nak
                                                                             (2, 3, 'member', 2),   -- Bob Wilson tag
                                                                             (3, 1, 'owner', 1),    -- John Doe tulajdonosa a Dashboard-nak
                                                                             (4, 1, 'owner', 1),    -- John Doe tulajdonosa az API Gateway-nek
                                                                             (4, 4, 'member', 1),   -- Alice Johnson tag
                                                                             (5, 5, 'owner', 5);    -- Mike Brown tulajdonosa a DevOps projektnek

-- Project assignment permissions hozzáadása
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
                                                                                ('project_assignments.create', 'Assign Users to Projects', 'Can assign users to projects', 'project_assignments', 'create'),
                                                                                ('project_assignments.read', 'View Project Assignments', 'Can view project assignments', 'project_assignments', 'read'),
                                                                                ('project_assignments.update', 'Update Project Assignments', 'Can update project assignments', 'project_assignments', 'update'),
                                                                                ('project_assignments.delete', 'Remove Project Assignments', 'Can remove users from projects', 'project_assignments', 'delete');

-- Jogok hozzárendelése szerepkörökhöz
-- Super Admin és Admin kapja az összes jogot
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin') AND p.name LIKE 'project_assignments.%';

-- Manager csak olvashat és létrehozhat hozzárendeléseket
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('project_assignments.read', 'project_assignments.create');

-- User csak olvashat
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name = 'project_assignments.read';