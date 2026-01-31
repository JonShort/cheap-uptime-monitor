# Cheap Uptime Monitor

A minimal, serverless HTTPS endpoint health checker built on AWS. Checks your endpoints for 200 OK responses and alerts you via email when things go wrong.

## Features

- **Serverless** - Lambda + EventBridge, pay only for what you use
- **Simple config** - JSON file in S3, no database required
- **Email alerts** - SNS notifications when endpoints fail
- **Custom headers** - Support for authenticated health checks
- **Configurable** - Per-endpoint intervals and timeouts

## Cost

For typical usage (10 endpoints, 15-minute checks):
- ~2,880 Lambda invocations/month → **Free tier**
- S3 storage for config → **< $0.01/month**
- SNS email notifications → **Free**

## Quick Start

### Prerequisites

- AWS CLI configured (`aws login` or `aws configure`)
- Terraform 1.0+
- Go 1.22+
- Make

### Deploy

```bash
# 1. Export AWS credentials (if using aws login)
eval "$(aws configure export-credentials --format env)"

# 2. Create Terraform state bucket (first time only)
./scripts/bootstrap-state-bucket.sh

# 3. Build the Lambda
make build

# 4. Deploy infrastructure
cd terraform
terraform init
terraform plan
terraform apply -var="alert_email=you@example.com"
```

### Configure Endpoints

```bash
# Copy the example config
cp config/config.example.json config/config.json

# Edit with your endpoints
vim config/config.json

# Upload to S3
make upload-config
```

### Confirm Email Subscription

Check your inbox for an SNS subscription confirmation email and click the link.

## Configuration

Endpoints are defined in `config/config.json`:

```json
{
  "endpoints": [
    {
      "name": "Production API",
      "url": "https://api.example.com/health",
      "interval_minutes": 15,
      "timeout_seconds": 30,
      "headers": {
        "Authorization": "Bearer your-token"
      }
    }
  ]
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | - | Display name for alerts |
| `url` | Yes | - | HTTPS URL to check |
| `interval_minutes` | No | 15 | Check frequency (1-1440) |
| `timeout_seconds` | No | 30 | Request timeout (1-60) |
| `headers` | No | {} | Custom HTTP headers |

See [Architecture/config-schema.md](Architecture/config-schema.md) for the full schema.

## Health Check Behavior

- **Success:** HTTP 200 only (not 201, 204, etc.)
- **Redirects:** Not followed (3xx = failure)
- **Timeout:** 30 seconds default
- **TLS:** Required (HTTPS only)

## Alert Format

When an endpoint fails, you'll receive an email like:

```
Subject: ALERT: Production API is DOWN

Uptime Monitor Alert

Endpoint: Production API
URL: https://api.example.com/health
Status: DOWN
Reason: unexpected status code: 503
Timestamp: 2024-01-15 10:30:00 UTC

--
Cheap Uptime Monitor
```

## Commands

```bash
make build          # Build Lambda binary
make test           # Run tests
make upload-config  # Upload config to S3
make clean          # Remove build artifacts
```

## Tear Down

```bash
cd terraform
terraform destroy

# Optionally remove state bucket
aws s3 rb s3://cheap-uptime-monitor-tf-state --force
```

## Architecture

```
┌──────────────┐     ┌────────────────┐     ┌─────────────────┐
│  EventBridge │────▶│     Lambda     │────▶│  Your Endpoints │
│  (schedule)  │     │   (Go binary)  │     │   (HTTPS GET)   │
└──────────────┘     └───────┬────────┘     └─────────────────┘
                             │
                    ┌────────┴────────┐
                    ▼                 ▼
             ┌──────────┐      ┌──────────┐
             │    S3    │      │   SNS    │
             │ (config) │      │ (alerts) │
             └──────────┘      └────┬─────┘
                                    │
                                    ▼
                               ┌─────────┐
                               │  Email  │
                               └─────────┘
```

See [Architecture/](Architecture/) for ADRs and detailed diagrams.

## Security

- S3 buckets block all public access
- Encryption at rest (AES256)
- IAM follows least-privilege principle
- TLS enforced for state bucket
- No credentials in code

## License

MIT
