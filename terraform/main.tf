terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Remote state storage - create bucket first using scripts/bootstrap-state-bucket.sh
  backend "s3" {
    bucket = "cheap-uptime-monitor-tf-state"
    key    = "terraform.tfstate"
    region = "eu-west-1"
  }
}

provider "aws" {
  # Region is configured via AWS_REGION environment variable
}

locals {
  prefix = "cheap-uptime-monitor"
}

# -----------------------------------------------------------------------------
# S3 Bucket for Configuration
# -----------------------------------------------------------------------------

resource "aws_s3_bucket" "config" {
  bucket = "${local.prefix}-config-bucket"
}

resource "aws_s3_bucket_public_access_block" "config" {
  bucket = aws_s3_bucket.config.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "config" {
  bucket = aws_s3_bucket.config.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "config" {
  bucket = aws_s3_bucket.config.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# -----------------------------------------------------------------------------
# SNS Topic for Alerts
# -----------------------------------------------------------------------------

resource "aws_sns_topic" "alerts" {
  name = "${local.prefix}-alerts"
}

resource "aws_sns_topic_subscription" "email" {
  count     = var.alert_email != "" ? 1 : 0
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# -----------------------------------------------------------------------------
# IAM Role for Lambda
# -----------------------------------------------------------------------------

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${local.prefix}-lambda-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "lambda_permissions" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # S3 - Read config and read/write known health
  statement {
    actions = [
      "s3:GetObject",
      "s3:PutObject",
    ]
    resources = [
      "${aws_s3_bucket.config.arn}/*",
    ]
  }

  # SNS - Publish alerts
  statement {
    actions = [
      "sns:Publish",
    ]
    resources = [
      aws_sns_topic.alerts.arn,
    ]
  }
}

resource "aws_iam_role_policy" "lambda" {
  name   = "${local.prefix}-lambda-policy"
  role   = aws_iam_role.lambda.id
  policy = data.aws_iam_policy_document.lambda_permissions.json
}

# -----------------------------------------------------------------------------
# S3 Bucket Policy - Restrict to Lambda Role
# -----------------------------------------------------------------------------

data "aws_iam_policy_document" "config_bucket_policy" {
  statement {
    sid    = "AllowLambdaAccess"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.lambda.arn]
    }

    actions = [
      "s3:GetObject",
      "s3:PutObject",
    ]

    resources = [
      "${aws_s3_bucket.config.arn}/*",
    ]
  }
}

resource "aws_s3_bucket_policy" "config" {
  bucket = aws_s3_bucket.config.id
  policy = data.aws_iam_policy_document.config_bucket_policy.json
}

# -----------------------------------------------------------------------------
# Lambda Function
# -----------------------------------------------------------------------------

resource "aws_lambda_function" "monitor" {
  function_name = "${local.prefix}-lambda"
  role          = aws_iam_role.lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = 60
  memory_size   = 128

  filename         = "${path.module}/../build/lambda.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/lambda.zip")

  environment {
    variables = {
      CONFIG_BUCKET = aws_s3_bucket.config.id
      CONFIG_KEY    = var.config_key
      HEALTH_KEY    = var.health_key
      SNS_TOPIC_ARN = aws_sns_topic.alerts.arn
    }
  }
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${aws_lambda_function.monitor.function_name}"
  retention_in_days = 14
}

# -----------------------------------------------------------------------------
# EventBridge Scheduler
# -----------------------------------------------------------------------------

data "aws_iam_policy_document" "scheduler_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name               = "${local.prefix}-scheduler-role"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume_role.json
}

data "aws_iam_policy_document" "scheduler_permissions" {
  statement {
    actions = [
      "lambda:InvokeFunction",
    ]
    resources = [
      aws_lambda_function.monitor.arn,
    ]
  }
}

resource "aws_iam_role_policy" "scheduler" {
  name   = "${local.prefix}-scheduler-policy"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler_permissions.json
}

resource "aws_scheduler_schedule" "monitor" {
  name       = "${local.prefix}-schedule"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = "rate(${var.check_interval_minutes} minutes)"

  target {
    arn      = aws_lambda_function.monitor.arn
    role_arn = aws_iam_role.scheduler.arn
  }
}
