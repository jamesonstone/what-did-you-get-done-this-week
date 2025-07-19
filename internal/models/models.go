package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type User struct {
	ID               int        `json:"id" db:"id"`
	Email            string     `json:"email" db:"email"`
	Name             string     `json:"name" db:"name"`
	Timezone         string     `json:"timezone" db:"timezone"`
	PromptTime       time.Time  `json:"prompt_time" db:"prompt_time"`
	VerificationCode *string    `json:"verification_code,omitempty" db:"verification_code"`
	IsVerified       bool       `json:"is_verified" db:"is_verified"`
	IsPaused         bool       `json:"is_paused" db:"is_paused"`
	PauseUntil       *time.Time `json:"pause_until,omitempty" db:"pause_until"`
	ProjectFocus     *string    `json:"project_focus,omitempty" db:"project_focus"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

type Entry struct {
	ID             int       `json:"id" db:"id"`
	UserID         int       `json:"user_id" db:"user_id"`
	EntryDate      time.Time `json:"entry_date" db:"entry_date"`
	RawContent     string    `json:"raw_content" db:"raw_content"`
	ParsedContent  *string   `json:"parsed_content,omitempty" db:"parsed_content"`
	ProjectTag     *string   `json:"project_tag,omitempty" db:"project_tag"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type WeeklySummary struct {
	ID               int           `json:"id" db:"id"`
	UserID           int           `json:"user_id" db:"user_id"`
	WeekStartDate    time.Time     `json:"week_start_date" db:"week_start_date"`
	SummaryParagraph string        `json:"summary_paragraph" db:"summary_paragraph"`
	BulletPoints     BulletPoints  `json:"bullet_points" db:"bullet_points"`
	LLMModel         string        `json:"llm_model" db:"llm_model"`
	LLMCostCents     int           `json:"llm_cost_cents" db:"llm_cost_cents"`
	CreatedAt        time.Time     `json:"created_at" db:"created_at"`
}

type EmailLog struct {
	ID             int        `json:"id" db:"id"`
	UserID         *int       `json:"user_id,omitempty" db:"user_id"`
	RecipientEmail string     `json:"recipient_email" db:"recipient_email"`
	EmailType      string     `json:"email_type" db:"email_type"`
	Subject        string     `json:"subject" db:"subject"`
	BodyText       string     `json:"body_text" db:"body_text"`
	Status         string     `json:"status" db:"status"`
	SESMessageID   *string    `json:"ses_message_id,omitempty" db:"ses_message_id"`
	ErrorMessage   *string    `json:"error_message,omitempty" db:"error_message"`
	RetryCount     int        `json:"retry_count" db:"retry_count"`
	ScheduledAt    *time.Time `json:"scheduled_at,omitempty" db:"scheduled_at"`
	SentAt         *time.Time `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// BulletPoints is a custom type for JSON array handling
type BulletPoints []string

func (bp BulletPoints) Value() (driver.Value, error) {
	return json.Marshal(bp)
}

func (bp *BulletPoints) Scan(value interface{}) error {
	if value == nil {
		*bp = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("cannot scan BulletPoints from non-string type")
	}

	return json.Unmarshal(bytes, bp)
}

// Email types constants
const (
	EmailTypeVerification   = "verification"
	EmailTypeDailyPrompt    = "daily_prompt"
	EmailTypeWeeklySummary  = "weekly_summary"
	EmailTypeClarification  = "clarification"
)

// Email statuses constants
const (
	EmailStatusPending  = "pending"
	EmailStatusSent     = "sent"
	EmailStatusFailed   = "failed"
	EmailStatusRetrying = "retrying"
)