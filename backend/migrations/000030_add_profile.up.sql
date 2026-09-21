-- backend/migrations/000030_add_profile.up.sql

CREATE TABLE profiles (
    id SERIAL PRIMARY KEY,
    background TEXT NOT NULL DEFAULT '',
    expertise TEXT NOT NULL DEFAULT '',
    tone_rules TEXT NOT NULL DEFAULT '',
    updated_by INTEGER REFERENCES users(id),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO profiles (id, background, expertise, tone_rules) VALUES (1, '', '', '');

CREATE TABLE profile_samples (
    id SERIAL PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_profile_samples_profile_id ON profile_samples(profile_id);
