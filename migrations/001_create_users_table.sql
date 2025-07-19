-- Users table: stores user information including email, name, timezone, preferred prompt time, verification, and pause state
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(50) NOT NULL, -- IANA timezone identifier (e.g., America/New_York)
    prompt_time TIME NOT NULL DEFAULT '16:00:00', -- Daily prompt time in user's local timezone
    verification_code VARCHAR(10),
    is_verified BOOLEAN DEFAULT FALSE,
    is_paused BOOLEAN DEFAULT FALSE,
    pause_until TIMESTAMP,
    project_focus VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient email lookups
CREATE INDEX idx_users_email ON users(email);

-- Index for verified users
CREATE INDEX idx_users_verified ON users(is_verified);

-- Index for scheduling (find users who need prompts)
CREATE INDEX idx_users_scheduling ON users(is_verified, is_paused, prompt_time);