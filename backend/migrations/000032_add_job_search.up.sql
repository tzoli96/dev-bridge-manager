-- backend/migrations/000032_add_job_search.up.sql

CREATE TABLE job_search_profiles (
    id SERIAL PRIMARY KEY,
    cv_text TEXT NOT NULL DEFAULT '',
    skills TEXT NOT NULL DEFAULT '',
    preferences TEXT NOT NULL DEFAULT '',
    updated_by INTEGER NOT NULL REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO job_search_profiles (id, updated_by) VALUES (1, 1);

CREATE TABLE job_listings (
    id SERIAL PRIMARY KEY,
    site VARCHAR(50) NOT NULL,
    external_url TEXT NOT NULL,
    title VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL,
    location VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL,
    posted_at TIMESTAMPTZ,
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (site, external_url)
);

CREATE TABLE job_matches (
    id SERIAL PRIMARY KEY,
    job_listing_id INTEGER NOT NULL UNIQUE REFERENCES job_listings(id) ON DELETE CASCADE,
    score INTEGER NOT NULL,
    reasoning TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'new',
    applied_at TIMESTAMPTZ,
    application_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_job_matches_score ON job_matches(score DESC);
