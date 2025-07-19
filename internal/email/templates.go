package email

import (
	"bytes"
	"embed"
	"fmt"
	"math/rand"
	"text/template"
	"time"
)

//go:embed ../../templates/*.txt
var templateFS embed.FS

type TemplateData struct {
	// Welcome email
	VerificationCode string

	// Daily prompt
	DayOfWeek    string
	Date         string
	ProjectFocus string
	Quote        string

	// Weekly summary
	WeekStart         string
	WeekEnd           string
	SummaryParagraph  string
	BulletPoints      []string

	// Clarification
	OriginalMessage string
}

var quotes = []string{
	"The way to get started is to quit talking and begin doing. - Walt Disney",
	"Innovation distinguishes between a leader and a follower. - Steve Jobs",
	"Your limitation—it's only your imagination.",
	"Push yourself, because no one else is going to do it for you.",
	"Great things never come from comfort zones.",
	"Dream it. Wish it. Do it.",
	"Success doesn't just find you. You have to go out and get it.",
	"The harder you work for something, the greater you'll feel when you achieve it.",
	"Don't stop when you're tired. Stop when you're done.",
	"Wake up with determination. Go to bed with satisfaction.",
}

func RenderWelcomeEmail(verificationCode string) (string, string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/welcome.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to parse welcome template: %w", err)
	}

	data := TemplateData{
		VerificationCode: verificationCode,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("failed to execute welcome template: %w", err)
	}

	subject := "Welcome to What Did You Get Done This Week?"
	return subject, buf.String(), nil
}

func RenderDailyPromptEmail(projectFocus *string) (string, string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/daily_prompt.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to parse daily prompt template: %w", err)
	}

	now := time.Now()
	data := TemplateData{
		DayOfWeek: now.Format("Monday"),
		Date:      now.Format("January 2, 2006"),
		Quote:     quotes[rand.Intn(len(quotes))],
	}

	if projectFocus != nil {
		data.ProjectFocus = *projectFocus
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("failed to execute daily prompt template: %w", err)
	}

	subject := fmt.Sprintf("What did you get done today? - %s", now.Format("Jan 2"))
	return subject, buf.String(), nil
}

func RenderWeeklySummaryEmail(weekStart time.Time, summaryParagraph string, bulletPoints []string) (string, string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/weekly_summary.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to parse weekly summary template: %w", err)
	}

	weekEnd := weekStart.AddDate(0, 0, 4) // Friday
	data := TemplateData{
		WeekStart:        weekStart.Format("Jan 2"),
		WeekEnd:          weekEnd.Format("Jan 2"),
		SummaryParagraph: summaryParagraph,
		BulletPoints:     bulletPoints,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("failed to execute weekly summary template: %w", err)
	}

	subject := fmt.Sprintf("This is What I Did This Week - %s", weekStart.Format("Jan 2"))
	return subject, buf.String(), nil
}

func RenderClarificationEmail(originalMessage string) (string, string, error) {
	tmpl, err := template.ParseFS(templateFS, "../../templates/clarification.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to parse clarification template: %w", err)
	}

	data := TemplateData{
		OriginalMessage: originalMessage,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("failed to execute clarification template: %w", err)
	}

	subject := "Clarification needed for your journal entry"
	return subject, buf.String(), nil
}

func GenerateVerificationCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}