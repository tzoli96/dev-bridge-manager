-- backend/migrations/000031_add_draft_feedback.up.sql

CREATE TABLE draft_feedback (
    id SERIAL PRIMARY KEY,
    email_id INTEGER NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
    ai_draft_text TEXT NOT NULL,
    sent_text TEXT NOT NULL,
    similarity DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_draft_feedback_email_id ON draft_feedback(email_id);
