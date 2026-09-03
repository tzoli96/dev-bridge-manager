-- 000011_create_kanban_tables.up.sql

CREATE TABLE kanban_columns (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(100) NOT NULL,
    color VARCHAR(30) NOT NULL DEFAULT 'bg-gray-500',
    position INTEGER NOT NULL DEFAULT 0,
    max_tasks INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_kanban_columns_project_id ON kanban_columns(project_id);

CREATE TABLE tasks (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    column_id INTEGER NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    html_description TEXT,
    priority VARCHAR(20) NOT NULL DEFAULT 'medium',
    status VARCHAR(20) NOT NULL DEFAULT 'todo',
    assignee_id INTEGER REFERENCES users(id),
    estimated_hours NUMERIC(6,2) NOT NULL DEFAULT 0,
    tags JSONB NOT NULL DEFAULT '[]',
    position INTEGER NOT NULL DEFAULT 0,
    due_date DATE,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_by INTEGER NOT NULL REFERENCES users(id),
    updated_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_tasks_project_id ON tasks(project_id);
CREATE INDEX idx_tasks_column_id ON tasks(column_id);
CREATE INDEX idx_tasks_assignee_id ON tasks(assignee_id);

CREATE TABLE task_comments (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    html_content TEXT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_task_comments_task_id ON task_comments(task_id);

CREATE TABLE task_time_entries (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    hours NUMERIC(6,2) NOT NULL,
    description TEXT,
    date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_task_time_entries_task_id ON task_time_entries(task_id);

-- Permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('tasks.create', 'Create Tasks', 'Can create tasks on project boards', 'tasks', 'create'),
    ('tasks.update', 'Update Tasks', 'Can edit tasks', 'tasks', 'update'),
    ('tasks.delete', 'Delete Tasks', 'Can delete tasks', 'tasks', 'delete'),
    ('tasks.move', 'Move Tasks', 'Can move tasks between columns', 'tasks', 'move'),
    ('kanban.manage_columns', 'Manage Columns', 'Can manage kanban board columns', 'kanban', 'manage_columns'),
    ('time_tracking.view', 'View Time Tracking', 'Can view logged time on tasks', 'time_tracking', 'view'),
    ('time_tracking.edit', 'Log Time', 'Can log and edit time entries', 'time_tracking', 'edit');

-- Task + time tracking permissions: super_admin, admin, manager, user
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin', 'manager', 'user')
  AND p.name IN ('tasks.create', 'tasks.update', 'tasks.delete', 'tasks.move', 'time_tracking.view', 'time_tracking.edit');

-- Column management: super_admin, admin, manager only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin', 'manager')
  AND p.name = 'kanban.manage_columns';
