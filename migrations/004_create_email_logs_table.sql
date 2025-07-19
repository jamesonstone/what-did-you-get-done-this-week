-- Email logs table: logs all sent emails with their status and timestamps (outbox pattern)
CREATE TABLE email_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    recipient_email VARCHAR(255) NOT NULL,
    email_type VARCHAR(50) NOT NULL, -- 'verification', 'daily_prompt', 'weekly_summary', 'clarification'
    subject VARCHAR(500) NOT NULL,
    body_text TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'sent', 'failed', 'retrying'
    ses_message_id VARCHAR(255), -- AWS SES message ID
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    scheduled_at TIMESTAMP,
    sent_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for status-based processing (outbox pattern)
CREATE INDEX idx_email_logs_status ON email_logs(status, scheduled_at);

-- Index for user-based queries
CREATE INDEX idx_email_logs_user ON email_logs(user_id);

-- Index for email type and date tracking
CREATE INDEX idx_email_logs_type_date ON email_logs(email_type, created_at);

-- Index for retry processing
CREATE INDEX idx_email_logs_retry ON email_logs(status, retry_count, created_at);