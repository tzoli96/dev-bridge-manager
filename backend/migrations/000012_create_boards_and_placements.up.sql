-- backend/migrations/000012_create_boards_and_placements.up.sql

CREATE TABLE boards (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_boards_project_id ON boards(project_id);

INSERT INTO boards (project_id, name, position, created_at, updated_at)
SELECT DISTINCT project_id, 'Main Board', 0, NOW(), NOW()
FROM kanban_columns;

ALTER TABLE kanban_columns ADD COLUMN board_id INTEGER;

UPDATE kanban_columns kc
SET board_id = b.id
FROM boards b
WHERE b.project_id = kc.project_id;

ALTER TABLE kanban_columns ALTER COLUMN board_id SET NOT NULL;
ALTER TABLE kanban_columns ADD CONSTRAINT kanban_columns_board_id_fkey FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE;
CREATE INDEX idx_kanban_columns_board_id ON kanban_columns(board_id);
ALTER TABLE kanban_columns DROP COLUMN project_id;

CREATE TABLE task_placements (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id INTEGER NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(task_id, board_id)
);

CREATE INDEX idx_task_placements_task_id ON task_placements(task_id);
CREATE INDEX idx_task_placements_board_id ON task_placements(board_id);
CREATE INDEX idx_task_placements_column_id ON task_placements(column_id);

INSERT INTO task_placements (task_id, board_id, column_id, position, created_at, updated_at)
SELECT t.id, kc.board_id, t.column_id, t.position, NOW(), NOW()
FROM tasks t
JOIN kanban_columns kc ON kc.id = t.column_id;

ALTER TABLE tasks DROP COLUMN column_id;
ALTER TABLE tasks DROP COLUMN position;
