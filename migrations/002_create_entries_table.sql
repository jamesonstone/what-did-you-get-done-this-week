-- Entries table: stores daily journal entries per user
CREATE TABLE entries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_date DATE NOT NULL,
    raw_content TEXT NOT NULL,
    parsed_content TEXT, -- Cleaned/parsed version of the content
    project_tag VARCHAR(255), -- Extracted project tag from content
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Unique constraint: one entry per user per day
CREATE UNIQUE INDEX idx_entries_user_date ON entries(user_id, entry_date);

-- Index for date-based queries
CREATE INDEX idx_entries_date ON entries(entry_date);

-- Index for user-based queries
CREATE INDEX idx_entries_user ON entries(user_id);