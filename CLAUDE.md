# CLAUDE.md

## Project Overview

Cheap Uptime Monitor is a serverless HTTPS endpoint health checker built on AWS. It checks configured endpoints for 200 OK responses and alerts via SNS when failures occur.

## Tech Stack

- **Runtime:** AWS Lambda with Go (1.22+, `provided.al2023` runtime)
- **Scheduling:** AWS EventBridge Scheduler
- **Configuration:** S3 bucket with JSON file
- **Alerts:** AWS SNS (email subscriptions)
- **Infrastructure:** Terraform
- **State:** Stateless (no persistence of check results)

## Code Expectations

### Go

- Use modern Go (1.22+) idioms and features
- Use `net/http` standard library for HTTP client (no external HTTP libraries needed)
- Use AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`)
- Use `provided.al2023` Lambda runtime with `github.com/aws/aws-lambda-go`
- Keep dependencies minimal
- Handle errors explicitly, no panics in production code
- Use structured logging

### Terraform

- Use Terraform 1.0+ syntax
- All resources prefixed with `cheap-uptime-monitor-`
- Region configured via `AWS_REGION` environment variable
- Single environment (no dev/staging/prod separation)
- Keep it simple and readable

## Repository Structure

```
/
├── Architecture/        # ADRs, diagrams, schema docs
├── cmd/                 # Lambda entry point
├── internal/            # Internal Go packages
├── terraform/           # Terraform configuration
├── config/              # Sample configuration files
├── go.mod
├── go.sum
└── README.md
```

## Health Check Behavior

- **Success:** HTTP 200 status code only
- **Timeout:** 30 seconds default, configurable per endpoint
- **Redirects:** Do not follow (3xx = failure)
- **Custom headers:** Supported per endpoint
- **Check interval:** Set globally via Terraform (default 15 minutes)

## Alert Content

When an endpoint fails, the SNS message includes:
- Endpoint name
- URL
- Failure reason (timeout, connection error, non-200 status)
- Timestamp (UTC)

## Configuration

Endpoints are configured via JSON in S3. See `Architecture/config-schema.md` for the full schema.

```json
{
  "endpoints": [
    {
      "name": "Production API",
      "url": "https://api.example.com/health",
      "timeout_seconds": 30,
      "headers": {
        "X-Health-Check": "monitor"
      }
    }
  ]
}
```

## Security Requirements

- S3 config bucket: public access blocked, encrypted at rest (AES256), access restricted to Lambda role
- S3 state bucket: public access blocked, encrypted at rest, versioned, TLS enforced
- No hardcoded credentials in code or Terraform
- Sensitive header values are stored in S3 config (consider Secrets Manager for future enhancement)

## Key Decisions

All architectural decisions are documented in `Architecture/decision-records.md`. Key points:

1. Stateless design - alerts on every failure, no duplicate suppression
2. No recovery notifications (would require state)
3. Strict 200 OK requirement (not 2xx)
4. Don't follow redirects
5. Single Lambda checks all endpoints per invocation

## Development Commands

```bash
# Build Lambda binary
make build

# Run tests
make test

# First-time setup: create Terraform state bucket (run once)
./scripts/bootstrap-state-bucket.sh

# Deploy infrastructure
cd terraform
terraform init
terraform plan
terraform apply -var="alert_email=you@example.com"

# Update endpoint configuration
cp config/config.example.json config/config.json
# Edit config/config.json with your endpoints
make upload-config

# Tear down everything
cd terraform
terraform destroy
```

## AWS Authentication

If using `aws login`, export credentials before running Terraform:

```bash
eval "$(aws configure export-credentials --format env)"
```

This exports `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` to the current shell.

## Updating the Stack

### Endpoint Config Changes
Upload to S3 - takes effect on next scheduled run:
```bash
make upload-config
```

### Lambda Code Changes
Rebuild and redeploy (Terraform detects zip hash change):
```bash
make build
cd terraform && terraform apply
```

### Infrastructure Changes
Edit Terraform files, then:
```bash
cd terraform
terraform plan    # Preview
terraform apply   # Apply
```

| Change | Command |
|--------|---------|
| Add/remove endpoints | `make upload-config` |
| Update Lambda code | `make build && cd terraform && terraform apply` |
| Change check interval | `terraform apply -var="check_interval_minutes=N"` |
| Change alert email | `terraform apply -var="alert_email=..."` |

## Testing

- Unit tests for config parsing and health check logic
- Integration tests can use httptest for mock endpoints
- No external test dependencies required
