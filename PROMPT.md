## DOMAIN AND DEPLOYMENT

This app is deployed at `whatdidyougetdone.dev` — a domain purchased through Squarespace and configured to point to AWS services with minimal cost.

## GOALS

Let's create a new Golang backend application called "What Did You Get Done This Week?" — a joyful daily journaling service that works entirely through email.

This is a backend-only system: no UI, no frontend.

The user flow is elegant and low-friction: each user receives a personalized email once per day at their configured local time (e.g. 4pm). The email asks them: “What did you get done today?” — with context like the day of week, their current project name, and a fun quote.

The system properly handles daylight savings time using UTC conversion and IANA timezone identifiers collected during signup.

## DAILY EMAIL FLOW

- Each user receives a personalized daily prompt email at their configured local time.
- The email asks: “What did you get done today?” with contextual information (day of week, current project, fun quote).
- Users respond directly to the email with free text.
- Replies are parsed, logged, and stored in a Postgres database.
- Replies are associated with the user and date for future summaries.
- Free text parsing includes detection of opt-out commands and user-controlled pauses.
- The system ignores attachments and uses only plaintext email body content.
- Structured XML-style tags such as `<entry>`, `<pause>`, and `<project>` are supported.
- If reply parsing fails or is ambiguous, the system sends a clarification request.

## WEEKLY SUMMARY FLOW

- Each Friday at 4:30pm (or a user-configured time), the system sends a weekly digest email titled: “This is What I Did This Week.”
- The email contains a one-paragraph summary of the user’s accomplishments.
- It includes a bulleted list of the most important items from their daily entries, Monday through Friday.
- To generate this summary, the backend invokes a lightweight large language model (LLM) through AWS — such as Amazon Bedrock or a Lambda-integrated open-source model.
- The LLM takes the user's daily email replies and outputs a coherent summary.
- The LLM determines which items are most important and presents them as bullets under the summary paragraph.
- This email serves as both a motivational artifact and a record of progress.
- Summaries are cached per week to avoid duplicate charges.
- The tone is assertive and output-driven, reflecting a no-nonsense productivity ethos.

## USER SIGNUP FLOW

- Users sign up with a two-step email verification: they email a "Start" request.
- The system responds with a short verification code.
- The user must reply to confirm and activate their journaling account.
- This allows passwordless, email-only authentication.
- Users sign up by emailing a specific address (e.g., `start@whatdidyougetdone.com`) with the subject "Start" or by visiting a CLI script that sends a signup request to the backend.
- On signup, a welcome email is sent that asks the user to reply with their preferences using a terminal-style text prompt format.
- The system bootstraps the initial user using a hardcoded email for admin access.

## EMAIL TEMPLATES

- All email templates (welcome, daily prompt, weekly summary) are written in plaintext format using Go's `text/template` system.
- Markdown is optionally supported, but emails are designed to feel like terminal output.
- Templates are stored in-code.

The welcome email body looks like this:

```bash
+----------------------------------------------------------+
| Welcome to "What Did You Get Done This Week?" ✍️        |
|                                                          |
| Before we start sending your daily journaling prompts,   |
| please reply to this email with the following info:      |
|                                                          |
| 1. Name: ___________                                     |
| 2. Timezone (e.g., America/New_York): ___________        |
| 3. Preferred daily prompt time (e.g., 16:00): ___________|
| 4. Project focus tag (optional): ___________             |
|                                                          |
| That's it — we'll take care of the rest.                 |
+----------------------------------------------------------+
```

## INBOUND EMAIL HANDLING

- The system implements an outbox pattern for email delivery.
- Scheduled emails are written to the `email_logs` table.
- A separate delivery worker is responsible for sending and updating status (e.g., sent, failed).
- This enables retries and decouples email sending from business logic.
- AWS SES + S3 + Lambda are used for inbound email receipt and processing.
- AWS Lambda (or API Gateway + webhook endpoint) receives and parses replies (email via SES inbound).
- The system ignores attachments and uses only plaintext email body content.
- Supports structured XML-style tags such as `<entry>`, `<pause>`, and `<project>`.
- If reply parsing fails or is ambiguous, the system sends a polite clarification request asking the user to reformat their response.

## CLI TOOL

The CLI tool supports the following commands:

- `verify resend` — triggers resend of verification code
- `config show` — prints current user config for debugging
- `email trigger-daily` — manually triggers the daily prompt
- `email trigger-weekly` — manually triggers the weekly summary

## DATABASE SCHEMA

| Table            | Columns                                                                                  | Description                                                                                      |
|------------------|------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| users            | email, name, timezone (IANA), prompt_time, verification status, pause state              | Stores user information including email, name, timezone, preferred prompt time, verification, and pause state |
| entries          | one record per user per day, storing the raw reply and timestamp                         | Stores daily journal entries per user                                                          |
| weekly_summaries | one per user per week, with a paragraph summary and 3–5 bulleted takeaways               | Stores generated weekly summaries                                                              |
| email_logs       | records of sent emails by type, status, and timestamp                                   | Logs all sent emails with their status and timestamps                                          |

## BUSINESS LOGIC RULES

- If a user doesn’t reply to a daily prompt, the system simply skips that day.
- Users can opt out temporarily by replying with:
  - “don’t ask me again this week”
  - “don’t ask me again for the next 3 days”
  - “i’m on vacation for the month so don’t ask me for this month”
- These commands are parsed and respected automatically.
- Reply format expects free text with optional commands.
- If parsing fails or is ambiguous, the system sends a polite clarification request asking the user to reformat their response.
- The system properly handles daylight savings time using UTC conversion and IANA timezone identifiers collected during signup.
- The experience has a flat, minimal terminal feel, as if interacting with a text-based shell system.

## SCHEDULING AND JOBS

- The core app runs as a container and includes:
  1. A scheduler that checks which users need to be emailed at the current time and sends the daily email.
  2. A receiver that accepts incoming replies (via webhook or polling) and logs the content.
  3. A PostgreSQL schema with tables for users, entries, and email logs.
  4. Structured logging, metrics, and graceful error handling.
- Scheduling uses `gocron` or `robfig/cron` to schedule emails.

## LLM INTEGRATION

The prompt used for weekly summarization is:

```bash
System: Summarize the user’s weekly journal in Elon Musk's tone. Extract top 3–5 priorities as bullet points.

User:
Monday: ...
Tuesday: ...
...
Friday: ...
```

- The backend invokes a lightweight large language model (LLM) through AWS — such as Amazon Bedrock or a Lambda-integrated open-source model.
- The summary is generated via the lowest-cost LLM integration available, called inline from Go using the Amazon Bedrock SDK or similar.
- The LLM takes all five weekday responses at once and generates:
  - Summary: A single Elon-style paragraph
  - Bullets: Top 3–5 most important takeaways
- Tone is assertive and output-driven, reflecting a no-nonsense productivity ethos.
- Summaries are cached per week to avoid duplicate charges.
- LLM prompts and responses are logged to CloudWatch when running in single-user mode for review.
- Parse errors are logged and flagged in metrics; retry logic may be triggered when necessary.

## INFRASTRUCTURE & DEPLOYMENT

- Golang is used for the backend.
- Postgres is used as the database.
- Docker Compose is used for local development.
- AWS SES is used for email delivery.
- AWS SES + S3 + Lambda are used for inbound email receipt and processing.
- AWS Lambda (or API Gateway + webhook endpoint) receives and parses replies.
- The domain `whatdidyougetdone.dev` is purchased via Squarespace, configured for HTTPS and email.
- AWS Certificate Manager (ACM) provides free TLS certificates (required for .dev domains).
- Squarespace DNS is used for CNAME and TXT records (to avoid AWS Route53 fees).
- Free-tier AWS services are used wherever possible (e.g. CloudWatch, Lambda, ACM).
- Terraform is used to provision all AWS infrastructure: SES, S3, Lambda, IAM, CloudWatch, and ACM.
- Local development is containerized with Docker Compose.
- Production deployments follow infrastructure-as-code principles with Terraform modules for each subsystem.
- GitHub Actions are used for CI and Docker health checks.
- Basic admin API access is secured via static key.
- Architecture is multi-user ready but configured for single-user initial deployment.

## MONITORING & LOGGING

- Structured logging and metrics are implemented.
- LLM prompts and responses are logged to CloudWatch in single-user mode.
- Parse errors are logged and flagged in metrics.
- Retry logic may be triggered when necessary.

## CONFIGURATION EXAMPLE (.env)

```bash
# Domain and Email
DOMAIN=whatdidyougetdone.dev
EMAIL_FROM=no-reply@whatdidyougetdone.com
SIGNUP_EMAIL=start@whatdidyougetdone.com

# AWS Configurations
AWS_REGION=us-east-1
AWS_SES_REGION=us-east-1
AWS_S3_BUCKET=your-email-bucket
AWS_LAMBDA_FUNCTION=your-lambda-function-name

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=youruser
POSTGRES_PASSWORD=yourpassword
POSTGRES_DB=whatdidyougetdone

# Scheduler
DEFAULT_PROMPT_TIME=16:00
WEEKLY_SUMMARY_TIME=16:30

# Admin API
ADMIN_API_KEY=your_static_admin_api_key

# LLM Integration
LLM_PROVIDER=amazon_bedrock
LLM_MODEL=lowest_cost_model
```

## README DOCUMENTATION

The repository must include a clearly written `README.md` file. This should cover:

- Overview of the application
- Architecture diagram or summary
- How to run the app locally using Docker Compose
- How to run individual components (scheduler, parser, CLI)
- How to connect to the database
- Required environment variables and `.env` setup
- How to deploy to AWS using Terraform
- How to test inbound email and trigger the LLM summary
- Sample email flows (signup, daily prompt, weekly summary)

## IMPLEMENTATION CHECKLIST

To implement this application fully, complete the following in order:

1. Set up the Go monorepo project structure with modules for: core, scheduler, parser, CLI, and API
2. Create SQL migrations for the full schema (`users`, `entries`, `email_logs`, `weekly_summaries`)
3. Build the daily prompt scheduler using `gocron`, reading per-user config from Postgres
4. Implement the outbox pattern for queued email sending via SES
5. Set up inbound email parsing via SES → S3 → Lambda → Go parser
6. Add structured reply parsing (XML-style tags + free text fallbacks)
7. Add CLI utility with subcommands (verify resend, config show, email trigger-*)
8. Implement two-step email verification logic
9. Integrate AWS Bedrock or cheapest LLM option for weekly summaries
10. Log all LLM requests/responses to CloudWatch
11. Use Docker Compose for local dev (DB, mail relay/test harness, Go services)
12. Write a full Terraform module to deploy SES, Lambda, ACM, and DNS config
13. Add GitHub Actions for testing and deployment
14. Create `README.md` with all setup instructions and architectural documentation
