package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

type DB struct {
	*sql.DB
}

func New(cfg *config.Config) (*DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logrus.Info("Database connection established")
	return &DB{db}, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) RunMigrations() error {
	migrations := []string{
		`-- Users table
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			timezone VARCHAR(50) NOT NULL,
			prompt_time TIME NOT NULL DEFAULT '16:00:00',
			verification_code VARCHAR(10),
			is_verified BOOLEAN DEFAULT FALSE,
			is_paused BOOLEAN DEFAULT FALSE,
			pause_until TIMESTAMP,
			project_focus VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_verified ON users(is_verified);
		CREATE INDEX IF NOT EXISTS idx_users_scheduling ON users(is_verified, is_paused, prompt_time);`,

		`-- Entries table
		CREATE TABLE IF NOT EXISTS entries (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			entry_date DATE NOT NULL,
			raw_content TEXT NOT NULL,
			parsed_content TEXT,
			project_tag VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_user_date ON entries(user_id, entry_date);
		CREATE INDEX IF NOT EXISTS idx_entries_date ON entries(entry_date);
		CREATE INDEX IF NOT EXISTS idx_entries_user ON entries(user_id);`,

		`-- Weekly summaries table
		CREATE TABLE IF NOT EXISTS weekly_summaries (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			week_start_date DATE NOT NULL,
			summary_paragraph TEXT NOT NULL,
			bullet_points JSON NOT NULL,
			llm_model VARCHAR(100) NOT NULL,
			llm_cost_cents INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_weekly_summaries_user_week ON weekly_summaries(user_id, week_start_date);
		CREATE INDEX IF NOT EXISTS idx_weekly_summaries_date ON weekly_summaries(week_start_date);
		CREATE INDEX IF NOT EXISTS idx_weekly_summaries_user ON weekly_summaries(user_id);`,

		`-- Email logs table
		CREATE TABLE IF NOT EXISTS email_logs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			recipient_email VARCHAR(255) NOT NULL,
			email_type VARCHAR(50) NOT NULL,
			subject VARCHAR(500) NOT NULL,
			body_text TEXT NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			ses_message_id VARCHAR(255),
			error_message TEXT,
			retry_count INTEGER DEFAULT 0,
			scheduled_at TIMESTAMP,
			sent_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_email_logs_status ON email_logs(status, scheduled_at);
		CREATE INDEX IF NOT EXISTS idx_email_logs_user ON email_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_email_logs_type_date ON email_logs(email_type, created_at);
		CREATE INDEX IF NOT EXISTS idx_email_logs_retry ON email_logs(status, retry_count, created_at);`,
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", i+1, err)
		}
	}

	logrus.Info("Database migrations completed successfully")
	return nil
}