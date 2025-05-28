CREATE TABLE role_permissions (
                                  id SERIAL PRIMARY KEY,
                                  role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
                                  permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
                                  created_at TIMESTAMP DEFAULT NOW(),
                                  UNIQUE(role_id, permission_id)
);

-- Create indexes
CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON role_permissions(permission_id);

-- Assign permissions to roles
-- Super Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'super_admin';

-- Admin gets most permissions except system management
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name NOT LIKE 'system%';

-- Manager gets user read/update and own profile
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('users.read', 'users.list', 'users.update', 'profile.read', 'profile.update');

-- User gets only profile permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name IN ('profile.read', 'profile.update');

-- Guest gets only profile read
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'guest' AND p.name = 'profile.read';