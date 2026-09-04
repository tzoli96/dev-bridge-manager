-- backend/migrations/000014_create_task_activity_log_table.up.sql

CREATE TABLE task_activity_log (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    event_type VARCHAR(30) NOT NULL,
    field_name VARCHAR(50),
    old_value TEXT,
    new_value TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_activity_log_task_id ON task_activity_log(task_id);
