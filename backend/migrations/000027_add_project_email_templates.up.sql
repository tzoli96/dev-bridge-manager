CREATE TABLE project_email_templates (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    email_type VARCHAR(20) NOT NULL,
    subject VARCHAR(998) NOT NULL,
    body TEXT NOT NULL,
    updated_by INTEGER NOT NULL REFERENCES users(id),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, email_type)
);
