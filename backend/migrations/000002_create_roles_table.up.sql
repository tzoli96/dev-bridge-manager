CREATE TABLE roles (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(50) UNIQUE NOT NULL,
                       display_name VARCHAR(100) NOT NULL,
                       description TEXT,
                       is_active BOOLEAN DEFAULT true,
                       created_at TIMESTAMP DEFAULT NOW(),
                       updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default roles
INSERT INTO roles (name, display_name, description) VALUES
                                                        ('super_admin', 'Super Admin', 'Full system access with all permissions'),
                                                        ('admin', 'Administrator', 'Administrative access to most features'),
                                                        ('manager', 'Manager', 'Management level access'),
                                                        ('user', 'User', 'Standard user access'),
                                                        ('guest', 'Guest', 'Limited read-only access');
