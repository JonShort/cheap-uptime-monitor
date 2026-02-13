# Cheap Uptime Monitor

A minimal, serverless HTTPS endpoint health checker built on AWS. Checks your endpoints for 200 OK responses and alerts you via email when things go wrong.

## Features

- **Serverless** - Lambda + EventBridge, pay only for what you use
- **Simple config** - JSON file in S3, no database required
- **Smart alerts** - DOWN and RECOVERED notifications only on state transitions (no duplicate alerts)
- **Custom headers** - Support for authenticated health checks
- **Configurable** - Per-endpoint timeouts and global check interval

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
| `timeout_seconds` | No | 30 | Request timeout (1-60) |
| `headers` | No | {} | Custom HTTP headers |

> **Note:** Check interval is set globally via Terraform (`check_interval_minutes`, default 15).

See [Architecture/config-schema.md](Architecture/config-schema.md) for the full schema.

## Health Check Behavior

- **Success:** HTTP 200 only (not 201, 204, etc.)
- **Redirects:** Not followed (3xx = failure)
- **Timeout:** 30 seconds default
- **TLS:** Required (HTTPS only)

## Alert Format

When an endpoint goes down, you'll receive:

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

When it recovers:

```
Subject: RECOVERED: Production API is UP

Uptime Monitor Recovery

Endpoint: Production API
URL: https://api.example.com/health
Status: UP
Timestamp: 2024-01-15 10:45:00 UTC

--
Cheap Uptime Monitor
```

Alerts only fire on state transitions — you won't get repeated emails while an endpoint stays down.

## Commands

```bash
make build          # Build Lambda binary
make test           # Run tests
make upload-config  # Upload config to S3
make clean          # Remove build artifacts
```

## Updating

### Endpoint Changes (add/remove/modify endpoints)

Edit `config/config.json` and upload - takes effect on next scheduled run:

```bash
make upload-config
```

### Code Changes

Rebuild and redeploy the Lambda:

```bash
make build
cd terraform && terraform apply
```

### Infrastructure Changes

| Change | Command |
|--------|---------|
| Check interval | `terraform apply -var="check_interval_minutes=5"` |
| Alert email | `terraform apply -var="alert_email=new@example.com"` |
| Other infra | Edit `terraform/*.tf`, then `terraform apply` |

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
