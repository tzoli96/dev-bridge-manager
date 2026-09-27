ALTER TABLE projects ADD COLUMN contract_end_date TIMESTAMP;

CREATE TABLE project_renewal_flags (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    contract_end_date TIMESTAMP NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    dismissed_by INTEGER REFERENCES users(id),
    dismissed_at TIMESTAMP
);

CREATE INDEX idx_project_renewal_flags_project ON project_renewal_flags (project_id);
CREATE INDEX idx_project_renewal_flags_status ON project_renewal_flags (status);
