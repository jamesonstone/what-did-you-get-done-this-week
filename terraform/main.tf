terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Variables
variable "domain" {
  description = "Domain name for the application"
  type        = string
  default     = "whatdidyougetdone.dev"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

# S3 bucket for email storage
resource "aws_s3_bucket" "email_storage" {
  bucket = "${var.domain}-email-storage-${var.environment}"
}

resource "aws_s3_bucket_versioning" "email_storage" {
  bucket = aws_s3_bucket.email_storage.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "email_storage" {
  bucket = aws_s3_bucket.email_storage.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# SES domain verification
resource "aws_ses_domain_identity" "main" {
  domain = var.domain
}

resource "aws_ses_domain_dkim" "main" {
  domain = aws_ses_domain_identity.main.domain
}

# SES email addresses
resource "aws_ses_email_identity" "no_reply" {
  email = "no-reply@${var.domain}"
}

resource "aws_ses_email_identity" "start" {
  email = "start@${var.domain}"
}

# SES receipt rule set
resource "aws_ses_receipt_rule_set" "main" {
  rule_set_name = "${var.domain}-ruleset"
}

resource "aws_ses_active_receipt_rule_set" "main" {
  rule_set_name = aws_ses_receipt_rule_set.main.rule_set_name
}

# SES receipt rule for inbound emails
resource "aws_ses_receipt_rule" "inbound" {
  name          = "inbound-email-rule"
  rule_set_name = aws_ses_receipt_rule_set.main.rule_set_name
  recipients    = [var.domain]
  enabled       = true
  scan_enabled  = true

  s3_action {
    bucket_name = aws_s3_bucket.email_storage.bucket
    object_key_prefix = "emails/"
    position = 1
  }

  lambda_action {
    function_arn = aws_lambda_function.email_parser.arn
    position = 2
  }
}

# IAM role for Lambda
resource "aws_iam_role" "lambda_execution" {
  name = "email-parser-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.lambda_execution.name
}

# Lambda permissions for S3 and SES
resource "aws_iam_role_policy" "lambda_permissions" {
  name = "email-parser-lambda-permissions"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject"
        ]
        Resource = "${aws_s3_bucket.email_storage.arn}/*"
      },
      {
        Effect = "Allow"
        Action = [
          "ses:SendEmail",
          "ses:SendRawEmail"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "bedrock:InvokeModel"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      }
    ]
  })
}

# Lambda function for email parsing
resource "aws_lambda_function" "email_parser" {
  filename         = "lambda-deployment.zip"
  function_name    = "email-parser"
  role            = aws_iam_role.lambda_execution.arn
  handler         = "bootstrap"
  runtime         = "provided.al2"
  timeout         = 30

  environment {
    variables = {
      POSTGRES_HOST     = var.postgres_host
      POSTGRES_PORT     = var.postgres_port
      POSTGRES_USER     = var.postgres_user
      POSTGRES_PASSWORD = var.postgres_password
      POSTGRES_DB       = var.postgres_db
      AWS_REGION        = var.aws_region
      AWS_SES_REGION    = var.aws_region
      EMAIL_FROM        = "no-reply@${var.domain}"
      LLM_PROVIDER      = "amazon_bedrock"
      LLM_MODEL         = "anthropic.claude-3-haiku-20240307-v1:0"
    }
  }

  depends_on = [aws_iam_role_policy_attachment.lambda_basic]
}

# Lambda permission for SES
resource "aws_lambda_permission" "ses_invoke" {
  statement_id  = "AllowExecutionFromSES"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.email_parser.function_name
  principal     = "ses.amazonaws.com"
  source_arn    = "arn:aws:ses:${var.aws_region}:${data.aws_caller_identity.current.account_id}:receipt-rule-set/${aws_ses_receipt_rule_set.main.rule_set_name}:receipt-rule/${aws_ses_receipt_rule.inbound.name}"
}

# ACM certificate for the domain
resource "aws_acm_certificate" "main" {
  domain_name       = var.domain
  validation_method = "DNS"

  subject_alternative_names = [
    "*.${var.domain}"
  ]

  lifecycle {
    create_before_destroy = true
  }
}

# CloudWatch Log Group for Lambda
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/email-parser"
  retention_in_days = 30
}

# Data sources
data "aws_caller_identity" "current" {}

# Database variables (would be provided via terraform.tfvars)
variable "postgres_host" {
  description = "PostgreSQL host"
  type        = string
  sensitive   = true
}

variable "postgres_port" {
  description = "PostgreSQL port"
  type        = string
  default     = "5432"
}

variable "postgres_user" {
  description = "PostgreSQL user"
  type        = string
  sensitive   = true
}

variable "postgres_password" {
  description = "PostgreSQL password"
  type        = string
  sensitive   = true
}

variable "postgres_db" {
  description = "PostgreSQL database name"
  type        = string
  default     = "whatdidyougetdone"
}

# Outputs
output "ses_domain_verification_record" {
  description = "DNS record for SES domain verification"
  value = {
    name  = "_amazonses.${var.domain}"
    type  = "TXT"
    value = aws_ses_domain_identity.main.verification_token
  }
}

output "ses_dkim_records" {
  description = "DNS records for SES DKIM"
  value = [
    for token in aws_ses_domain_dkim.main.dkim_tokens : {
      name  = "${token}._domainkey.${var.domain}"
      type  = "CNAME"
      value = "${token}.dkim.amazonses.com"
    }
  ]
}

output "acm_certificate_validation_records" {
  description = "DNS records for ACM certificate validation"
  value = [
    for dvo in aws_acm_certificate.main.domain_validation_options : {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  ]
}

output "s3_bucket_name" {
  description = "S3 bucket name for email storage"
  value       = aws_s3_bucket.email_storage.bucket
}

output "lambda_function_name" {
  description = "Lambda function name for email parsing"
  value       = aws_lambda_function.email_parser.function_name
}