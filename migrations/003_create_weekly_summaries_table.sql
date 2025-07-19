-- Weekly summaries table: stores generated weekly summaries
CREATE TABLE weekly_summaries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start_date DATE NOT NULL, -- Monday of the week
    summary_paragraph TEXT NOT NULL, -- Generated paragraph summary
    bullet_points JSON NOT NULL, -- Array of 3-5 key takeaways
    llm_model VARCHAR(100) NOT NULL, -- Model used for generation
    llm_cost_cents INTEGER DEFAULT 0, -- Cost tracking in cents
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Unique constraint: one summary per user per week
CREATE UNIQUE INDEX idx_weekly_summaries_user_week ON weekly_summaries(user_id, week_start_date);

-- Index for date-based queries
CREATE INDEX idx_weekly_summaries_date ON weekly_summaries(week_start_date);

-- Index for user-based queries
CREATE INDEX idx_weekly_summaries_user ON weekly_summaries(user_id);