-- backend/migrations/000015_add_subtasks_and_done_column.down.sql

ALTER TABLE kanban_columns DROP COLUMN is_done;

DROP INDEX IF EXISTS idx_tasks_parent_task_id;
ALTER TABLE tasks DROP COLUMN parent_task_id;
