package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/core"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/database"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/email"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/llm"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/models"
	"github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load config")
	}

	db, err := database.New(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		logrus.WithError(err).Fatal("Failed to run database migrations")
	}

	emailService, err := email.NewService(db, cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create email service")
	}

	coreService := core.NewService(db, emailService)

	llmService, err := llm.NewService(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create LLM service")
	}

	scheduler := gocron.NewScheduler(time.UTC)

	// Schedule daily prompts (run every hour to check for users)
	scheduler.Every(1).Hour().Do(func() {
		if err := sendDailyPrompts(context.Background(), coreService, emailService); err != nil {
			logrus.WithError(err).Error("Failed to send daily prompts")
		}
	})

	// Schedule weekly summaries (run every Friday at 4:30 PM UTC)
	scheduler.Every(1).Week().Friday().At("16:30").Do(func() {
		if err := sendWeeklySummaries(context.Background(), coreService, emailService, llmService); err != nil {
			logrus.WithError(err).Error("Failed to send weekly summaries")
		}
	})

	// Schedule email outbox processing (every 5 minutes)
	scheduler.Every(5).Minutes().Do(func() {
		if err := emailService.ProcessOutbox(context.Background()); err != nil {
			logrus.WithError(err).Error("Failed to process email outbox")
		}
	})

	scheduler.StartAsync()
	logrus.Info("Scheduler started")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logrus.Info("Shutting down scheduler...")
	scheduler.Stop()
}

func sendDailyPrompts(ctx context.Context, coreService *core.Service, emailService *email.Service) error {
	currentHour := time.Now().UTC().Hour()
	
	users, err := coreService.GetUsersForDailyPrompt(ctx, currentHour)
	if err != nil {
		return err
	}

	for _, user := range users {
		// Check if user's local time matches their preferred prompt time
		if shouldSendPrompt(user, currentHour) {
			err := emailService.SendDailyPrompt(ctx, user.ID, user.Email, user.ProjectFocus)
			if err != nil {
				logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to send daily prompt")
				continue
			}
			
			logrus.WithField("user_id", user.ID).Info("Daily prompt queued")
		}
	}

	return nil
}

func shouldSendPrompt(user *models.User, currentHour int) bool {
	// Load user's timezone
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		logrus.WithError(err).WithField("timezone", user.Timezone).Error("Invalid timezone")
		return false
	}

	// Get current time in user's timezone
	userTime := time.Now().In(loc)
	promptHour := user.PromptTime.Hour()

	return userTime.Hour() == promptHour
}

func sendWeeklySummaries(ctx context.Context, coreService *core.Service, emailService *email.Service, llmService *llm.Service) error {
	// Get all verified users
	users, err := getAllVerifiedUsers(ctx, coreService)
	if err != nil {
		return err
	}

	for _, user := range users {
		// Get entries for this week
		entries, err := getWeekEntries(ctx, coreService, user.ID)
		if err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to get week entries")
			continue
		}

		if len(entries) == 0 {
			logrus.WithField("user_id", user.ID).Info("No entries for this week, skipping summary")
			continue
		}

		// Generate summary using LLM
		summary, err := llmService.GenerateWeeklySummary(ctx, entries)
		if err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to generate weekly summary")
			continue
		}

		// Send summary email
		weekStart := getWeekStart()
		err = emailService.SendWeeklySummary(ctx, user.ID, user.Email, weekStart, 
			summary.Paragraph, summary.BulletPoints)
		if err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to send weekly summary")
			continue
		}

		// Save summary to database
		err = saveWeeklySummary(ctx, coreService, user.ID, weekStart, summary)
		if err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to save weekly summary")
		}

		logrus.WithField("user_id", user.ID).Info("Weekly summary sent")
	}

	return nil
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

// Placeholder functions that would need implementation
func getAllVerifiedUsers(ctx context.Context, coreService *core.Service) ([]*models.User, error) {
	// Implementation needed
	return nil, nil
}

func getWeekEntries(ctx context.Context, coreService *core.Service, userID int) ([]*models.Entry, error) {
	// Implementation needed
	return nil, nil
}

func saveWeeklySummary(ctx context.Context, coreService *core.Service, userID int, weekStart time.Time, summary *llm.WeeklySummary) error {
	// Implementation needed
	return nil
}