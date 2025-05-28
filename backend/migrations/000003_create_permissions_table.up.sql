CREATE TABLE permissions (
                             id SERIAL PRIMARY KEY,
                             name VARCHAR(100) UNIQUE NOT NULL,
                             display_name VARCHAR(150) NOT NULL,
                             description TEXT,
                             resource VARCHAR(50) NOT NULL, -- pl: users, projects, reports
                             action VARCHAR(50) NOT NULL,   -- pl: create, read, update, delete
                             is_active BOOLEAN DEFAULT true,
                             created_at TIMESTAMP DEFAULT NOW(),
                             updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
-- User management
('users.create', 'Create Users', 'Can create new users', 'users', 'create'),
('users.read', 'View Users', 'Can view user information', 'users', 'read'),
('users.update', 'Update Users', 'Can update user information', 'users', 'update'),
('users.delete', 'Delete Users', 'Can delete users', 'users', 'delete'),
('users.list', 'List Users', 'Can list all users', 'users', 'list'),

-- Role management
('roles.create', 'Create Roles', 'Can create new roles', 'roles', 'create'),
('roles.read', 'View Roles', 'Can view role information', 'roles', 'read'),
('roles.update', 'Update Roles', 'Can update roles', 'roles', 'update'),
('roles.delete', 'Delete Roles', 'Can delete roles', 'roles', 'delete'),
('roles.list', 'List Roles', 'Can list all roles', 'roles', 'list'),

-- Profile management
('profile.read', 'View Own Profile', 'Can view own profile', 'profile', 'read'),
('profile.update', 'Update Own Profile', 'Can update own profile', 'profile', 'update'),

-- System management
('system.settings', 'System Settings', 'Can manage system settings', 'system', 'manage'),
('system.logs', 'View System Logs', 'Can view system logs', 'system', 'read');