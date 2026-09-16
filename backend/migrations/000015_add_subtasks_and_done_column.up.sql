-- backend/migrations/000015_add_subtasks_and_done_column.up.sql

ALTER TABLE tasks ADD COLUMN parent_task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE;
CREATE INDEX idx_tasks_parent_task_id ON tasks(parent_task_id);

ALTER TABLE kanban_columns ADD COLUMN is_done BOOLEAN NOT NULL DEFAULT FALSE;
