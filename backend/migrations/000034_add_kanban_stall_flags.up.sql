CREATE TABLE kanban_stall_flags (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id INTEGER NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
    days_stalled INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    dismissed_by INTEGER REFERENCES users(id),
    dismissed_at TIMESTAMP
);

CREATE INDEX idx_kanban_stall_flags_task ON kanban_stall_flags (task_id);
CREATE INDEX idx_kanban_stall_flags_status ON kanban_stall_flags (status);
