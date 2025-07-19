package email

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/database"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/models"
	pkgConfig "github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

type Service struct {
	db        *database.DB
	sesClient *ses.Client
	config    *pkgConfig.Config
}

func NewService(db *database.DB, cfg *pkgConfig.Config) (*Service, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.AWSSESRegion))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Service{
		db:        db,
		sesClient: ses.NewFromConfig(awsCfg),
		config:    cfg,
	}, nil
}

func (s *Service) QueueEmail(ctx context.Context, userID *int, recipientEmail, emailType, subject, body string, scheduledAt *time.Time) error {
	query := `
		INSERT INTO email_logs (user_id, recipient_email, email_type, subject, body_text, scheduled_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.db.ExecContext(ctx, query, userID, recipientEmail, emailType, subject, body, scheduledAt)
	if err != nil {
		return fmt.Errorf("failed to queue email: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"user_id":    userID,
		"email_type": emailType,
		"recipient":  recipientEmail,
	}).Info("Email queued for delivery")

	return nil
}

func (s *Service) ProcessOutbox(ctx context.Context) error {
	query := `
		SELECT id, user_id, recipient_email, email_type, subject, body_text, retry_count
		FROM email_logs 
		WHERE status = 'pending' AND (scheduled_at IS NULL OR scheduled_at <= NOW())
		ORDER BY created_at ASC
		LIMIT 10`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query pending emails: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var email models.EmailLog
		err := rows.Scan(&email.ID, &email.UserID, &email.RecipientEmail, 
			&email.EmailType, &email.Subject, &email.BodyText, &email.RetryCount)
		if err != nil {
			logrus.WithError(err).Error("Failed to scan email log")
			continue
		}

		if err := s.sendEmail(ctx, &email); err != nil {
			logrus.WithError(err).WithField("email_id", email.ID).Error("Failed to send email")
			if err := s.markEmailFailed(ctx, email.ID, err.Error()); err != nil {
				logrus.WithError(err).Error("Failed to mark email as failed")
			}
		}
	}

	return nil
}

func (s *Service) sendEmail(ctx context.Context, email *models.EmailLog) error {
	input := &ses.SendEmailInput{
		Source: aws.String(s.config.EmailFrom),
		Destination: &types.Destination{
			ToAddresses: []string{email.RecipientEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(email.Subject),
			},
			Body: &types.Body{
				Text: &types.Content{
					Data: aws.String(email.BodyText),
				},
			},
		},
	}

	result, err := s.sesClient.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send email via SES: %w", err)
	}

	return s.markEmailSent(ctx, email.ID, *result.MessageId)
}

func (s *Service) markEmailSent(ctx context.Context, emailID int, messageID string) error {
	query := `
		UPDATE email_logs 
		SET status = 'sent', ses_message_id = $2, sent_at = NOW(), updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, emailID, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark email as sent: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"email_id":    emailID,
		"ses_msg_id":  messageID,
	}).Info("Email marked as sent")

	return nil
}

func (s *Service) markEmailFailed(ctx context.Context, emailID int, errorMsg string) error {
	query := `
		UPDATE email_logs 
		SET status = 'failed', error_message = $2, retry_count = retry_count + 1, updated_at = NOW()
		WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, emailID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark email as failed: %w", err)
	}

	return nil
}

func (s *Service) SendWelcomeEmail(ctx context.Context, recipientEmail, verificationCode string) error {
	subject, body, err := RenderWelcomeEmail(verificationCode)
	if err != nil {
		return fmt.Errorf("failed to render welcome email: %w", err)
	}

	return s.QueueEmail(ctx, nil, recipientEmail, models.EmailTypeVerification, subject, body, nil)
}

func (s *Service) SendDailyPrompt(ctx context.Context, userID int, recipientEmail string, projectFocus *string) error {
	subject, body, err := RenderDailyPromptEmail(projectFocus)
	if err != nil {
		return fmt.Errorf("failed to render daily prompt: %w", err)
	}

	return s.QueueEmail(ctx, &userID, recipientEmail, models.EmailTypeDailyPrompt, subject, body, nil)
}

func (s *Service) SendWeeklySummary(ctx context.Context, userID int, recipientEmail string, weekStart time.Time, summaryParagraph string, bulletPoints []string) error {
	subject, body, err := RenderWeeklySummaryEmail(weekStart, summaryParagraph, bulletPoints)
	if err != nil {
		return fmt.Errorf("failed to render weekly summary: %w", err)
	}

	return s.QueueEmail(ctx, &userID, recipientEmail, models.EmailTypeWeeklySummary, subject, body, nil)
}

func (s *Service) SendClarificationRequest(ctx context.Context, userID int, recipientEmail, originalMessage string) error {
	subject, body, err := RenderClarificationEmail(originalMessage)
	if err != nil {
		return fmt.Errorf("failed to render clarification email: %w", err)
	}

	return s.QueueEmail(ctx, &userID, recipientEmail, models.EmailTypeClarification, subject, body, nil)
}

// GetUserByEmail retrieves user from database
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, name, timezone, prompt_time, verification_code, is_verified, 
			   is_paused, pause_until, project_focus, created_at, updated_at
		FROM users WHERE email = $1`

	var user models.User
	var pauseUntil sql.NullTime
	var verificationCode sql.NullString
	var projectFocus sql.NullString

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.Timezone, &user.PromptTime,
		&verificationCode, &user.IsVerified, &user.IsPaused, &pauseUntil,
		&projectFocus, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if verificationCode.Valid {
		user.VerificationCode = &verificationCode.String
	}
	if pauseUntil.Valid {
		user.PauseUntil = &pauseUntil.Time
	}
	if projectFocus.Valid {
		user.ProjectFocus = &projectFocus.String
	}

	return &user, nil
}