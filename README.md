# What Did You Get Done This Week?

Inspired by Elon Musk's legendary "What did you get done this week?" challenge to the Twitter CEO, this service brings radical accountability to your inbox. Get daily prompts and AI-powered weekly summaries—no apps, no distractions, just results. All through email.

## 🚀 Features

- **Email-Only Interface**: No UI, no frontend - everything happens through email
- **Daily Prompts**: Personalized emails at your preferred time with motivational quotes
- **Weekly AI Summaries**: Elon Musk-style summaries generated using AWS Bedrock
- **Timezone Support**: Proper timezone handling with daylight savings time
- **Pause Controls**: Users can pause prompts for days, weeks, or months
- **Project Tracking**: Optional project focus tags for better organization
- **Outbox Pattern**: Reliable email delivery with retry logic
- **Two-Step Verification**: Secure passwordless authentication

## 🏗️ Architecture

```bash
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Email User    │────│  AWS SES     │────│   S3 Bucket     │
└─────────────────┘    └──────────────┘    └─────────────────┘
                               │                     │
                               │                     │
                               ▼                     ▼
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Scheduler     │────│  Go Backend  │────│ Lambda Parser   │
│   (gocron)      │    │  (Core Logic)│    │ (Email Processing)|
└─────────────────┘    └──────────────┘    └─────────────────┘
                               │
                               ▼
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   PostgreSQL    │────│  AWS Bedrock │────│   CloudWatch    │
│   (Database)    │    │  (LLM)       │    │   (Logging)     │
└─────────────────┘    └──────────────┘    └─────────────────┘
```

## 📁 Project Structure

```bash
├── cmd/
│   ├── scheduler/          # Daily/weekly email scheduler
│   ├── parser/             # Lambda function for inbound emails
│   └── cli/                # Command-line management tool
├── internal/
│   ├── core/               # Business logic and email parsing
│   ├── database/           # Database connection and migrations
│   ├── email/              # Email templates and SES integration
│   ├── llm/                # AWS Bedrock integration
│   └── models/             # Data models
├── pkg/
│   └── config/             # Configuration management
├── templates/              # Email templates
├── migrations/             # SQL migrations
├── terraform/              # Infrastructure as code
└── docker/                 # Docker configurations
```

## 🛠️ Local Development

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- AWS CLI configured (for production features)

### Setup

1. **Clone and setup environment:**

   ```bash
   git clone https://github.com/jamesonstone/what-did-you-get-done-this-week.git
   cd what-did-you-get-done-this-week
   make env  # Create .env from template
   ```

2. **Start development services:**

   ```bash
   make dev-setup
   ```

   This starts:
   - PostgreSQL (localhost:5432)
   - MailHog for email testing (`http://localhost:8025`)
   - LocalStack for AWS services (`http://localhost:4566`)

3. **Build and run components:**

   ```bash
   make build          # Build all binaries
   make cli            # Build CLI only
   make scheduler      # Build scheduler only
   ```

### Using the CLI

```bash
# Run database migrations
./bin/cli db migrate

# Create a new user
./bin/cli user signup user@example.com

# Send daily prompt manually
./bin/cli email trigger-daily user@example.com

# Send weekly summary manually
./bin/cli email trigger-weekly user@example.com

# List all users
./bin/cli user list

# Show user configuration
./bin/cli config show user@example.com

# Process email outbox
./bin/cli email process-outbox
```

### Testing Email Flow

1. **View emails in MailHog:** `http://localhost:8025`
2. **Simulate inbound email processing:** Use the CLI to trigger emails manually
3. **Test signup flow:** Use `./bin/cli user signup test@example.com`

## 📧 Email Workflows

### Signup Flow

1. User emails `start@whatdidyougetdone.com` with subject "Start"
2. System sends welcome email with verification code
3. User replies with preferences (name, timezone, prompt time, project)
4. System activates account and begins daily prompts

### Daily Prompt Flow

1. Scheduler checks every hour for users whose local time matches their preferred prompt time
2. Sends personalized email with day, date, project focus, and motivational quote
3. User replies with free text or structured commands:
   - `<pause>3 days</pause>` - Pause prompts
   - `<project>New Project</project>` - Update project focus
   - Plain text - Journal entry

### Weekly Summary Flow

1. Every Friday at 4:30 PM (configurable), system collects user's entries from Monday-Friday
2. Calls AWS Bedrock with Elon Musk-style prompt
3. Generates summary paragraph + 3-5 bullet points
4. Emails summary with subject "This is What I Did This Week"

## 🔧 Configuration

### Environment Variables

```bash
# Domain and Email
DOMAIN=whatdidyougetdone.dev
EMAIL_FROM=no-reply@whatdidyougetdone.com
SIGNUP_EMAIL=start@whatdidyougetdone.com

# AWS Configuration
AWS_REGION=us-east-1
AWS_SES_REGION=us-east-1
AWS_S3_BUCKET=your-email-bucket
AWS_LAMBDA_FUNCTION=your-lambda-function-name

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=password
POSTGRES_DB=whatdidyougetdone

# Scheduler
DEFAULT_PROMPT_TIME=16:00
WEEKLY_SUMMARY_TIME=16:30

# LLM Integration
LLM_PROVIDER=amazon_bedrock
LLM_MODEL=anthropic.claude-3-haiku-20240307-v1:0
```

## 🌐 AWS Deployment

### Infrastructure Setup

1. **Configure Terraform:**

   ```bash
   cd terraform
   terraform init
   terraform plan
   terraform apply
   ```

2. **Deploy Lambda function:**

   ```bash
   # Build Lambda deployment package
   GOOS=linux go build -o bootstrap ./cmd/parser
   zip lambda-deployment.zip bootstrap

   # Deploy via AWS CLI or Terraform
   aws lambda update-function-code \
     --function-name email-parser \
     --zip-file fileb://lambda-deployment.zip
   ```

3. **Configure SES:**
   - Verify domain: `whatdidyougetdone.dev`
   - Set up inbound email rules to trigger Lambda
   - Configure DKIM and SPF records

### Production Deployment

1. **Build production images:**

   ```bash
   docker build -f docker/Dockerfile.scheduler -t scheduler:latest .
   ```

2. **Deploy to ECS/EKS/EC2:**
   - Use provided Docker images
   - Set environment variables for production
   - Ensure AWS credentials are available

## 📊 Monitoring

- **CloudWatch Logs**: Structured JSON logging for all components
- **Email Metrics**: Delivery rates, bounce handling via SES
- **LLM Costs**: Tracked per summary generation
- **Health Checks**: Database connectivity, AWS service availability

## 🧪 Testing

```bash
# Run all tests
make test

# Test specific modules
go test ./internal/core/...
go test ./internal/email/...

# Integration tests with Docker
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## 📝 Database Schema

### Users Table

- `id`, `email`, `name`, `timezone`, `prompt_time`
- `verification_code`, `is_verified`, `is_paused`, `pause_until`
- `project_focus`, `created_at`, `updated_at`

### Entries Table

- `id`, `user_id`, `entry_date`, `raw_content`, `parsed_content`
- `project_tag`, `created_at`, `updated_at`

### Weekly Summaries Table

- `id`, `user_id`, `week_start_date`, `summary_paragraph`
- `bullet_points` (JSON), `llm_model`, `llm_cost_cents`

### Email Logs Table (Outbox Pattern)

- `id`, `user_id`, `recipient_email`, `email_type`, `subject`, `body_text`
- `status`, `ses_message_id`, `error_message`, `retry_count`
- `scheduled_at`, `sent_at`, `created_at`, `updated_at`

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## ⚡ Quick Start

```bash
# Clone and setup
git clone https://github.com/jamesonstone/what-did-you-get-done-this-week.git
cd what-did-you-get-done-this-week

# Start local development
make dev-setup

# Create your first user
./bin/cli user signup your-email@example.com

# Check MailHog for the welcome email
open http://localhost:8025
```

---

Built with ❤️ for productivity enthusiasts who believe in the power of daily reflection and weekly accountability.
