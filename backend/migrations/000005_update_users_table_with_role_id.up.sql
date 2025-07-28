-- Add role_id column to users table
ALTER TABLE users ADD COLUMN role_id INTEGER REFERENCES roles(id);

-- Update existing users with default role
UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'user');

-- Update John to be admin
UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'admin')
WHERE email = 'john@example.com';

-- Make role_id NOT NULL after setting defaults
ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;

-- Remove old role column
ALTER TABLE users DROP COLUMN IF EXISTS role;

-- Create index
CREATE INDEX idx_users_role_id ON users(role_id);