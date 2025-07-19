package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/core"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/database"
	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/email"
	"github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

type EmailData struct {
	From    string `json:"from"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func main() {
	lambda.Start(handleSESEvent)
}

func handleSESEvent(ctx context.Context, sesEvent events.SESEvent) error {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Error("Failed to load config")
		return err
	}

	db, err := database.New(cfg)
	if err != nil {
		logrus.WithError(err).Error("Failed to connect to database")
		return err
	}
	defer db.Close()

	emailService, err := email.NewService(db, cfg)
	if err != nil {
		logrus.WithError(err).Error("Failed to create email service")
		return err
	}

	coreService := core.NewService(db, emailService)

	for _, record := range sesEvent.Records {
		if err := processEmailRecord(ctx, coreService, record); err != nil {
			logrus.WithError(err).Error("Failed to process email record")
			continue
		}
	}

	return nil
}

func processEmailRecord(ctx context.Context, coreService *core.Service, record events.SESEventRecord) error {
	ses := record.SES
	mail := ses.Mail

	logrus.WithFields(logrus.Fields{
		"message_id": mail.MessageID,
		"timestamp":  mail.Timestamp,
		"source":     mail.Source,
	}).Info("Processing inbound email")

	// Extract sender email
	senderEmail := mail.Source
	if senderEmail == "" {
		return fmt.Errorf("no sender email found")
	}

	// Get email content from S3 (if stored there) or from the SES event
	emailData, err := extractEmailContent(record)
	if err != nil {
		return fmt.Errorf("failed to extract email content: %w", err)
	}

	// Process the email reply
	err = coreService.HandleEmailReply(ctx, senderEmail, emailData.Subject, emailData.Body)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"sender":     senderEmail,
			"subject":    emailData.Subject,
			"message_id": mail.MessageID,
		}).Error("Failed to handle email reply")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"sender":     senderEmail,
		"message_id": mail.MessageID,
	}).Info("Email reply processed successfully")

	return nil
}

func extractEmailContent(record events.SESEventRecord) (*EmailData, error) {
	ses := record.SES
	mail := ses.Mail

	// For now, we'll extract basic info from the SES event
	// In a full implementation, you'd retrieve the raw email from S3
	emailData := &EmailData{
		From:    mail.Source,
		Subject: "Daily Journal Reply", // Would be extracted from the actual email
		Body:    "",                    // Would be extracted from the actual email
	}

	// If the email has been stored in S3, we would:
	// 1. Parse the S3 object key from the SES event
	// 2. Download the raw email from S3
	// 3. Parse the email content (subject, body, etc.)
	
	// For this example, we'll look for content in the SES event itself
	// Note: SES events don't contain the full email body by default
	
	// This is a simplified version - in production you'd implement
	// proper email parsing from S3
	if len(record.SES.Receipt.Action.S3Action.BucketName) > 0 {
		// Email was stored in S3, would retrieve and parse it here
		logrus.Info("Email stored in S3, would retrieve and parse")
	}

	return emailData, nil
}

// Alternative HTTP handler for webhook-based email processing
func handleWebhook(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Error("Failed to load config")
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	db, err := database.New(cfg)
	if err != nil {
		logrus.WithError(err).Error("Failed to connect to database")
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}
	defer db.Close()

	emailService, err := email.NewService(db, cfg)
	if err != nil {
		logrus.WithError(err).Error("Failed to create email service")
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	coreService := core.NewService(db, emailService)

	// Parse webhook payload
	var emailData EmailData
	if err := json.Unmarshal([]byte(request.Body), &emailData); err != nil {
		logrus.WithError(err).Error("Failed to parse webhook payload")
		return events.APIGatewayProxyResponse{StatusCode: 400}, err
	}

	// Process the email
	err = coreService.HandleEmailReply(ctx, emailData.From, emailData.Subject, emailData.Body)
	if err != nil {
		logrus.WithError(err).Error("Failed to handle email reply")
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"status": "success"}`,
	}, nil
}