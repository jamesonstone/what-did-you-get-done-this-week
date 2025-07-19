package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	// Domain and Email
	Domain      string
	EmailFrom   string
	SignupEmail string

	// AWS
	AWSRegion       string
	AWSSESRegion    string
	AWSS3Bucket     string
	AWSLambdaFunc   string

	// Database
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	// Scheduler
	DefaultPromptTime   string
	WeeklySummaryTime   string

	// Admin
	AdminAPIKey string

	// LLM
	LLMProvider string
	LLMModel    string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		logrus.WithError(err).Debug("No .env file found, using environment variables")
	}

	port, err := strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Domain:      getEnv("DOMAIN", "whatdidyougetdone.dev"),
		EmailFrom:   getEnv("EMAIL_FROM", "no-reply@whatdidyougetdone.com"),
		SignupEmail: getEnv("SIGNUP_EMAIL", "start@whatdidyougetdone.com"),

		AWSRegion:     getEnv("AWS_REGION", "us-east-1"),
		AWSSESRegion:  getEnv("AWS_SES_REGION", "us-east-1"),
		AWSS3Bucket:   getEnv("AWS_S3_BUCKET", ""),
		AWSLambdaFunc: getEnv("AWS_LAMBDA_FUNCTION", ""),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     port,
		PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", ""),
		PostgresDB:       getEnv("POSTGRES_DB", "whatdidyougetdone"),

		DefaultPromptTime: getEnv("DEFAULT_PROMPT_TIME", "16:00"),
		WeeklySummaryTime: getEnv("WEEKLY_SUMMARY_TIME", "16:30"),

		AdminAPIKey: getEnv("ADMIN_API_KEY", ""),

		LLMProvider: getEnv("LLM_PROVIDER", "amazon_bedrock"),
		LLMModel:    getEnv("LLM_MODEL", "anthropic.claude-3-haiku-20240307-v1:0"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}