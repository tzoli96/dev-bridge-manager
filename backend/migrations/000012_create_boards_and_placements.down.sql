-- backend/migrations/000012_create_boards_and_placements.down.sql

ALTER TABLE tasks ADD COLUMN column_id INTEGER;
ALTER TABLE tasks ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

UPDATE tasks t
SET column_id = fp.column_id, position = fp.position
FROM (
    SELECT DISTINCT ON (task_id) task_id, column_id, position
    FROM task_placements
    ORDER BY task_id, id ASC
) fp
WHERE fp.task_id = t.id;

DELETE FROM tasks WHERE column_id IS NULL;

ALTER TABLE tasks ALTER COLUMN column_id SET NOT NULL;
ALTER TABLE tasks ADD CONSTRAINT tasks_column_id_fkey FOREIGN KEY (column_id) REFERENCES kanban_columns(id) ON DELETE CASCADE;
CREATE INDEX idx_tasks_column_id ON tasks(column_id);

DROP TABLE IF EXISTS task_placements;

ALTER TABLE kanban_columns ADD COLUMN project_id INTEGER;

UPDATE kanban_columns kc
SET project_id = b.project_id
FROM boards b
WHERE b.id = kc.board_id;

ALTER TABLE kanban_columns ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE kanban_columns ADD CONSTRAINT kanban_columns_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
CREATE INDEX idx_kanban_columns_project_id ON kanban_columns(project_id);
ALTER TABLE kanban_columns DROP COLUMN board_id;

DROP TABLE IF EXISTS boards;
