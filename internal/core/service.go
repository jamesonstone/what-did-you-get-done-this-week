package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/database"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/email"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/models"
)

type Service struct {
	db           *database.DB
	emailService *email.Service
}

func NewService(db *database.DB, emailService *email.Service) *Service {
	return &Service{
		db:           db,
		emailService: emailService,
	}
}

func (s *Service) HandleSignupRequest(ctx context.Context, emailAddr string) error {
	// Check if user already exists
	existingUser, err := s.emailService.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil && existingUser.IsVerified {
		return fmt.Errorf("user already exists and is verified")
	}

	// Generate verification code
	verificationCode := email.GenerateVerificationCode()

	if existingUser != nil {
		// Update existing user with new verification code
		err = s.updateUserVerificationCode(ctx, existingUser.ID, verificationCode)
	} else {
		// Create new user
		err = s.createPendingUser(ctx, emailAddr, verificationCode)
	}

	if err != nil {
		return fmt.Errorf("failed to create/update user: %w", err)
	}

	// Send welcome email with verification code
	return s.emailService.SendWelcomeEmail(ctx, emailAddr, verificationCode)
}

func (s *Service) HandleEmailReply(ctx context.Context, senderEmail, subject, body string) error {
	user, err := s.emailService.GetUserByEmail(ctx, senderEmail)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		// New user signup attempt
		if NeedsVerification(body) {
			return s.HandleSignupRequest(ctx, senderEmail)
		}
		return fmt.Errorf("unknown sender, please sign up first")
	}

	if !user.IsVerified {
		// Handle verification process
		return s.handleVerificationReply(ctx, user, body)
	}

	// Parse the reply
	parsed := ParseEmailReply(body)
	if !parsed.IsValidated {
		logrus.WithError(parsed.Error).WithField("user_id", user.ID).Error("Failed to parse email reply")
		return s.emailService.SendClarificationRequest(ctx, user.ID, user.Email, body)
	}

	// Process commands
	for _, cmd := range parsed.Commands {
		switch cmd.Type {
		case CommandTypePause:
			err = s.pauseUser(ctx, user.ID, *cmd.Duration)
		case CommandTypeProject:
			err = s.updateUserProject(ctx, user.ID, cmd.Value)
		case CommandTypeEntry:
			err = s.saveEntry(ctx, user.ID, cmd.Value, parsed.ProjectTag)
		}

		if err != nil {
			logrus.WithError(err).WithField("command_type", cmd.Type).Error("Failed to process command")
			return s.emailService.SendClarificationRequest(ctx, user.ID, user.Email, body)
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id":       user.ID,
		"commands_count": len(parsed.Commands),
	}).Info("Successfully processed email reply")

	return nil
}

func (s *Service) handleVerificationReply(ctx context.Context, user *models.User, body string) error {
	// Look for verification code in the reply
	if user.VerificationCode == nil {
		return fmt.Errorf("no verification code set for user")
	}

	// Simple check if the verification code is in the body
	if !contains(body, *user.VerificationCode) {
		return s.emailService.SendClarificationRequest(ctx, user.ID, user.Email, 
			"Please include your verification code in your reply")
	}

	// Parse user preferences from the reply
	preferences, err := parseUserPreferences(body)
	if err != nil {
		return s.emailService.SendClarificationRequest(ctx, user.ID, user.Email, 
			"Please provide your preferences in the format shown in the welcome email")
	}

	// Update user with preferences and mark as verified
	return s.verifyUser(ctx, user.ID, preferences)
}

func (s *Service) createPendingUser(ctx context.Context, email, verificationCode string) error {
	query := `
		INSERT INTO users (email, name, timezone, verification_code)
		VALUES ($1, $2, $3, $4)`

	_, err := s.db.ExecContext(ctx, query, email, "New User", "UTC", verificationCode)
	return err
}

func (s *Service) updateUserVerificationCode(ctx context.Context, userID int, verificationCode string) error {
	query := `
		UPDATE users 
		SET verification_code = $2, updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, userID, verificationCode)
	return err
}

func (s *Service) verifyUser(ctx context.Context, userID int, prefs *UserPreferences) error {
	query := `
		UPDATE users 
		SET name = $2, timezone = $3, prompt_time = $4, project_focus = $5, 
		    is_verified = TRUE, verification_code = NULL, updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, userID, prefs.Name, prefs.Timezone, 
		prefs.PromptTime, prefs.ProjectFocus)
	return err
}

func (s *Service) pauseUser(ctx context.Context, userID int, duration time.Duration) error {
	pauseUntil := time.Now().Add(duration)
	query := `
		UPDATE users 
		SET is_paused = TRUE, pause_until = $2, updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, userID, pauseUntil)
	return err
}

func (s *Service) updateUserProject(ctx context.Context, userID int, projectName string) error {
	query := `
		UPDATE users 
		SET project_focus = $2, updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, userID, projectName)
	return err
}

func (s *Service) saveEntry(ctx context.Context, userID int, content string, projectTag *string) error {
	today := time.Now().UTC().Format("2006-01-02")
	
	query := `
		INSERT INTO entries (user_id, entry_date, raw_content, parsed_content, project_tag)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, entry_date) 
		DO UPDATE SET raw_content = $3, parsed_content = $4, project_tag = $5, updated_at = NOW()`

	_, err := s.db.ExecContext(ctx, query, userID, today, content, content, projectTag)
	return err
}

func (s *Service) GetUsersForDailyPrompt(ctx context.Context, currentHour int) ([]*models.User, error) {
	query := `
		SELECT id, email, name, timezone, prompt_time, project_focus
		FROM users 
		WHERE is_verified = TRUE 
		  AND (is_paused = FALSE OR pause_until < NOW())
		  AND EXTRACT(HOUR FROM prompt_time) = $1`

	rows, err := s.db.QueryContext(ctx, query, currentHour)
	if err != nil {
		return nil, fmt.Errorf("failed to query users for daily prompt: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var projectFocus sql.NullString

		err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Timezone, 
			&user.PromptTime, &projectFocus)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if projectFocus.Valid {
			user.ProjectFocus = &projectFocus.String
		}

		users = append(users, &user)
	}

	return users, nil
}

func contains(text, substr string) bool {
	return len(text) > 0 && len(substr) > 0 && 
		   strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}