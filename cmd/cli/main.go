package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/core"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/database"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/email"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/llm"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/models"
	"github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

var (
	cfg          *config.Config
	db           *database.DB
	emailService *email.Service
	coreService  *core.Service
	llmService   *llm.Service
)

func main() {
	var err error
	
	cfg, err = config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load config")
	}

	db, err = database.New(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	emailService, err = email.NewService(db, cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create email service")
	}

	coreService = core.NewService(db, emailService)

	llmService, err = llm.NewService(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create LLM service")
	}

	rootCmd := &cobra.Command{
		Use:   "whatdidyougetdone",
		Short: "CLI for What Did You Get Done This Week journaling service",
		Long:  "Command line interface for managing the What Did You Get Done This Week email journaling service",
	}

	// Verify subcommands
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verification related commands",
	}

	verifyCmd.AddCommand(&cobra.Command{
		Use:   "resend [email]",
		Short: "Resend verification code to user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resendVerification(args[0])
		},
	})

	// Config subcommands
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration related commands",
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "show [email]",
		Short: "Show user configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return showUserConfig(args[0])
		},
	})

	// Email subcommands
	emailCmd := &cobra.Command{
		Use:   "email",
		Short: "Email related commands",
	}

	emailCmd.AddCommand(&cobra.Command{
		Use:   "trigger-daily [email]",
		Short: "Manually trigger daily prompt for user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return triggerDailyPrompt(args[0])
		},
	})

	emailCmd.AddCommand(&cobra.Command{
		Use:   "trigger-weekly [email]",
		Short: "Manually trigger weekly summary for user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return triggerWeeklySummary(args[0])
		},
	})

	emailCmd.AddCommand(&cobra.Command{
		Use:   "process-outbox",
		Short: "Process pending emails in outbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			return processOutbox()
		},
	})

	// User management subcommands
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	userCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listUsers()
		},
	})

	userCmd.AddCommand(&cobra.Command{
		Use:   "signup [email]",
		Short: "Initiate signup process for new user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return initiateSignup(args[0])
		},
	})

	// Database subcommands
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database related commands",
	}

	dbCmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrations()
		},
	})

	rootCmd.AddCommand(verifyCmd, configCmd, emailCmd, userCmd, dbCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func resendVerification(email string) error {
	ctx := context.Background()
	
	user, err := emailService.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", email)
	}

	if user.IsVerified {
		fmt.Printf("User %s is already verified\n", email)
		return nil
	}

	// Generate new verification code
	verificationCode := email.GenerateVerificationCode()
	
	// Update user with new code
	query := `UPDATE users SET verification_code = $2, updated_at = NOW() WHERE id = $1`
	_, err = db.ExecContext(ctx, query, user.ID, verificationCode)
	if err != nil {
		return fmt.Errorf("failed to update verification code: %w", err)
	}

	// Send welcome email
	err = emailService.SendWelcomeEmail(ctx, email, verificationCode)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	fmt.Printf("Verification email sent to %s\n", email)
	return nil
}

func showUserConfig(email string) error {
	ctx := context.Background()
	
	user, err := emailService.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", email)
	}

	userJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	fmt.Println(string(userJSON))
	return nil
}

func triggerDailyPrompt(email string) error {
	ctx := context.Background()
	
	user, err := emailService.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", email)
	}

	if !user.IsVerified {
		return fmt.Errorf("user is not verified: %s", email)
	}

	err = emailService.SendDailyPrompt(ctx, user.ID, user.Email, user.ProjectFocus)
	if err != nil {
		return fmt.Errorf("failed to send daily prompt: %w", err)
	}

	fmt.Printf("Daily prompt sent to %s\n", email)
	return nil
}

func triggerWeeklySummary(email string) error {
	ctx := context.Background()
	
	user, err := emailService.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", email)
	}

	if !user.IsVerified {
		return fmt.Errorf("user is not verified: %s", email)
	}

	// Get user's entries for this week
	entries, err := getUserWeekEntries(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get user entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Printf("No entries found for user %s this week\n", email)
		return nil
	}

	// Generate summary
	summary, err := llmService.GenerateWeeklySummary(ctx, entries)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Send summary email
	weekStart := getWeekStart()
	err = emailService.SendWeeklySummary(ctx, user.ID, user.Email, weekStart, 
		summary.Paragraph, summary.BulletPoints)
	if err != nil {
		return fmt.Errorf("failed to send weekly summary: %w", err)
	}

	fmt.Printf("Weekly summary sent to %s\n", email)
	return nil
}

func processOutbox() error {
	ctx := context.Background()
	
	err := emailService.ProcessOutbox(ctx)
	if err != nil {
		return fmt.Errorf("failed to process outbox: %w", err)
	}

	fmt.Println("Email outbox processed")
	return nil
}

func listUsers() error {
	ctx := context.Background()
	
	query := `SELECT email, name, timezone, is_verified, is_paused, created_at FROM users ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	fmt.Printf("%-30s %-20s %-20s %-10s %-8s %s\n", "EMAIL", "NAME", "TIMEZONE", "VERIFIED", "PAUSED", "CREATED")
	fmt.Println(strings.Repeat("-", 100))

	for rows.Next() {
		var email, name, timezone, createdAt string
		var isVerified, isPaused bool
		
		err := rows.Scan(&email, &name, &timezone, &isVerified, &isPaused, &createdAt)
		if err != nil {
			return fmt.Errorf("failed to scan user: %w", err)
		}

		fmt.Printf("%-30s %-20s %-20s %-10t %-8t %s\n", 
			email, name, timezone, isVerified, isPaused, createdAt[:10])
	}

	return nil
}

func initiateSignup(email string) error {
	ctx := context.Background()
	
	err := coreService.HandleSignupRequest(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to initiate signup: %w", err)
	}

	fmt.Printf("Signup initiated for %s\n", email)
	return nil
}

func runMigrations() error {
	err := db.RunMigrations()
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("Database migrations completed")
	return nil
}

// Helper functions (would need proper implementation)
func getUserWeekEntries(ctx context.Context, userID int) ([]*models.Entry, error) {
	// Implementation would query entries for the current week
	return nil, nil
}

func getWeekStart() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}
	daysToMonday := weekday - 1
	monday := now.AddDate(0, 0, -daysToMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}