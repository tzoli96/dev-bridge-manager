CREATE TABLE jira_board_integrations (
    id SERIAL PRIMARY KEY,
    board_id INTEGER NOT NULL UNIQUE REFERENCES boards(id) ON DELETE CASCADE,
    base_url VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    api_token VARCHAR(255) NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    connected_by INTEGER NOT NULL REFERENCES users(id),
    connected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_sync_at TIMESTAMP,
    last_sync_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jira_board_integrations_board ON jira_board_integrations (board_id);

ALTER TABLE tasks ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'local';
ALTER TABLE tasks ADD COLUMN jira_issue_key VARCHAR(50);
ALTER TABLE tasks ADD COLUMN jira_synced_at TIMESTAMP;

ALTER TABLE kanban_columns ADD COLUMN jira_status_name VARCHAR(100);
