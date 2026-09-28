ALTER TABLE kanban_columns DROP COLUMN jira_status_name;

ALTER TABLE tasks DROP COLUMN jira_synced_at;
ALTER TABLE tasks DROP COLUMN jira_issue_key;
ALTER TABLE tasks DROP COLUMN source;

DROP TABLE IF EXISTS jira_board_integrations;
