# Architecture Decision Records

## ADR-001: Serverless Architecture with AWS Lambda

**Status:** Accepted
**Date:** 2026-01-31

### Context
We need a cheap, low-maintenance uptime monitoring solution for a handful of HTTPS endpoints.

### Decision
Use AWS Lambda triggered by EventBridge Scheduler for executing health checks.

### Rationale
- Pay-per-invocation model is cost-effective for infrequent checks (default 15-minute intervals)
- No servers to maintain or patch
- Built-in retry and error handling
- Scales automatically if endpoint count grows

### Consequences
- Cold start latency (mitigated by using Go)
- 15-minute maximum timeout for Lambda execution
- Dependent on AWS availability

---

## ADR-002: Go as Lambda Runtime Language

**Status:** Accepted
**Date:** 2026-01-31

### Context
Need to choose a language for the Lambda function that balances development speed, performance, and maintainability.

### Decision
Use modern Go (1.22+) with the `provided.al2023` runtime.

### Rationale
- Fastest cold starts among Lambda-supported languages
- Compiled binary means smaller deployment package
- Strong standard library for HTTP clients
- Excellent AWS SDK support
- Type safety reduces runtime errors

### Consequences
- Requires Go toolchain for development
- Cross-compilation needed for Lambda (GOOS=linux GOARCH=amd64)

---

## ADR-003: S3 for Configuration Storage

**Status:** Accepted
**Date:** 2026-01-31

### Context
Need to store endpoint configuration (URLs, intervals, headers) in a way that's easy to update and secure.

### Decision
Store configuration as a JSON file in a private S3 bucket, accessible only by the Lambda function via IAM role.

### Rationale
- Near-zero cost for small config files
- Easy to update without redeploying Lambda
- Native AWS IAM integration for security
- Versioning available if needed later
- Simple to backup and audit

### Consequences
- Additional S3 GetObject call on each Lambda invocation
- Configuration changes don't require deployment
- Must ensure bucket policy restricts access to Lambda role only

---

## ADR-004: AWS SNS for Alert Notifications

**Status:** Accepted
**Date:** 2026-01-31

### Context
Need to notify operators when an endpoint fails health checks.

### Decision
Publish failure alerts to an SNS topic, initially configured for email delivery.

### Rationale
- Decouples alerting from the monitoring Lambda
- Easy to add SMS, Slack, PagerDuty, or other integrations later via additional subscriptions
- Native AWS service with no additional cost beyond usage
- Supports multiple subscribers per topic

### Consequences
- Email alerts require subscription confirmation
- Duplicate alert suppression handled by known health tracking (see ADR-012)

---

## ADR-005: Stateless Design (Initial Implementation)

**Status:** Superseded by ADR-012
**Date:** 2026-01-31

### Context
Decide whether to track endpoint state (up/down history) or operate statelessly.

### Decision
Start with a stateless design - alert on every failure without tracking previous state.

### Rationale
- Simplest possible implementation
- No database costs or complexity
- Sufficient for initial use case with few endpoints
- Easy to add state later if needed

### Consequences
- Duplicate alerts if an endpoint stays down across multiple check intervals
- No recovery notifications (would require knowing previous state)
- No historical data for uptime reporting

### Future Considerations
- Add DynamoDB or S3-based state tracking if duplicate alerts become problematic
- Could use CloudWatch Metrics for uptime dashboards

---

## ADR-006: Terraform for Infrastructure as Code

**Status:** Accepted
**Date:** 2026-01-31

### Context
Need to define and deploy AWS infrastructure in a repeatable, version-controlled manner.

### Decision
Use Terraform with the AWS provider for all infrastructure management.

### Rationale
- Declarative syntax is readable and predictable
- Excellent AWS provider coverage
- State management built-in
- No abstraction layers to debug (compared to CDK)
- Wide community support and documentation

### Consequences
- Requires Terraform CLI installation
- State file must be managed (local initially, could move to S3 backend)
- Team members need Terraform knowledge

---

## ADR-007: Health Check Behavior

**Status:** Accepted
**Date:** 2026-01-31

### Context
Define what constitutes a "healthy" endpoint and how checks should behave.

### Decision
- **Success criteria:** HTTP 200 status code only
- **Timeout:** 30 seconds default, configurable per endpoint
- **Redirects:** Do not follow (3xx responses are treated as failures)
- **Custom headers:** Supported per endpoint for authentication or identification

### Rationale
- Strict 200 OK requirement catches misconfigured endpoints
- Not following redirects ensures we're checking the actual endpoint, not a redirect target
- 30-second timeout is generous but prevents Lambda from hanging
- Custom headers enable checking authenticated endpoints

### Consequences
- Endpoints must return exactly 200 (not 201, 204, etc.)
- Redirect-based architectures will need direct endpoint URLs
- Header values stored in S3 config (consider secrets management for sensitive tokens)

---

## ADR-008: Default Check Interval

**Status:** Accepted
**Date:** 2026-01-31

### Context
Balance between detection speed and cost/resource usage.

### Decision
Default to 15-minute check intervals, configurable per endpoint (minimum TBD).

### Rationale
- 15 minutes is sufficient for most non-critical monitoring
- Keeps Lambda invocation costs minimal
- Configurable allows critical endpoints to be checked more frequently

### Consequences
- Worst-case detection time is 15 minutes for default endpoints
- More frequent checks increase costs linearly
- EventBridge Scheduler supports cron expressions if needed

---

## ADR-009: Monorepo Structure

**Status:** Accepted
**Date:** 2026-01-31

### Context
Decide how to organize the Go Lambda code and Terraform configuration.

### Decision
Use a monorepo with both Go source and Terraform in the same repository.

### Rationale
- Simpler to maintain for a small project
- Infrastructure and code changes can be reviewed together
- Single version control history
- Easier CI/CD setup

### Consequences
- Repository will contain mixed file types
- Need clear directory structure to separate concerns

### Structure
```
/
├── Architecture/        # Documentation (ADRs, diagrams)
├── cmd/                 # Go Lambda entry point
├── internal/            # Go internal packages
├── terraform/           # Terraform configuration
├── config/              # Sample configuration files
├── go.mod
├── go.sum
└── README.md
```

---

## ADR-010: AWS Region Configuration

**Status:** Accepted
**Date:** 2026-01-31

### Context
Determine how to specify which AWS region resources are deployed to.

### Decision
Configure AWS region via environment variable (`AWS_REGION`), used by both Terraform and the Lambda function.

### Rationale
- Standard AWS SDK convention
- Allows same code/config to deploy to different regions
- No hardcoded region values in source code

### Consequences
- Must set `AWS_REGION` before running Terraform
- Lambda inherits region from its execution environment

---

## ADR-011: Resource Naming Convention

**Status:** Accepted
**Date:** 2026-01-31

### Context
Need consistent naming for AWS resources for identification and potential multi-deployment scenarios.

### Decision
Prefix all AWS resources with `cheap-uptime-monitor-`.

### Rationale
- Clear identification in AWS Console
- Enables resource grouping via tags
- Avoids naming collisions with other projects

### Examples
- `cheap-uptime-monitor-lambda`
- `cheap-uptime-monitor-config-bucket`
- `cheap-uptime-monitor-alerts-topic`
- `cheap-uptime-monitor-scheduler`

---

## ADR-012: S3-Based Known Health Tracking

**Status:** Accepted
**Date:** 2026-02-13

### Context
The stateless design (ADR-005) caused duplicate DOWN alerts when an endpoint stayed down across multiple checks, and made recovery notifications impossible.

### Decision
Track the last known health of each endpoint in a `known-health.json` file in the existing S3 config bucket. Alert only on state transitions:
- **UP/UNKNOWN to DOWN:** Send DOWN alert
- **DOWN to UP:** Send RECOVERED alert
- **Same status:** No alert
- **UNKNOWN to UP:** Record status, no alert (first run)

### Rationale
- Eliminates duplicate DOWN alerts for persistently failing endpoints
- Enables recovery notifications without a database
- Reuses the existing S3 config bucket (no new infrastructure)
- Minimal additional cost (one extra S3 read/write per invocation)
- Avoids "state" terminology — uses "known health" to reflect that this is a cache of last-observed status, not authoritative state

### Consequences
- Lambda IAM role now requires `s3:PutObject` in addition to `s3:GetObject`
- First invocation after deployment treats all endpoints as UNKNOWN
- If the known health file is deleted, the system resets gracefully (all endpoints become UNKNOWN)
- Supersedes ADR-005
