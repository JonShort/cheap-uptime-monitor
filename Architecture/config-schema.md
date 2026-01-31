# Configuration Schema

## Overview

The uptime monitor reads its configuration from a JSON file stored in S3. This document defines the schema and provides examples.

## Schema Definition

### Root Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `endpoints` | `Endpoint[]` | Yes | Array of endpoints to monitor |

### Endpoint Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Human-readable name for the endpoint (used in alerts) |
| `url` | `string` | Yes | - | Full HTTPS URL to check (must start with `https://`) |
| `interval_minutes` | `integer` | No | `15` | How often to check this endpoint (minimum: 1) |
| `timeout_seconds` | `integer` | No | `30` | Request timeout before marking as failed (minimum: 1, maximum: 60) |
| `headers` | `object` | No | `{}` | Custom HTTP headers to include in the request |

### Headers Object

| Field | Type | Description |
|-------|------|-------------|
| `<header-name>` | `string` | Any valid HTTP header name and value |

## JSON Schema (Draft-07)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://cheap-uptime-monitor/config-schema.json",
  "title": "Uptime Monitor Configuration",
  "description": "Configuration for the cheap-uptime-monitor Lambda function",
  "type": "object",
  "required": ["endpoints"],
  "properties": {
    "endpoints": {
      "type": "array",
      "description": "List of endpoints to monitor",
      "items": {
        "$ref": "#/definitions/endpoint"
      },
      "minItems": 1
    }
  },
  "definitions": {
    "endpoint": {
      "type": "object",
      "required": ["name", "url"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Human-readable name for the endpoint",
          "minLength": 1,
          "maxLength": 100
        },
        "url": {
          "type": "string",
          "description": "HTTPS URL to monitor",
          "pattern": "^https://",
          "format": "uri"
        },
        "interval_minutes": {
          "type": "integer",
          "description": "Check interval in minutes",
          "default": 15,
          "minimum": 1,
          "maximum": 1440
        },
        "timeout_seconds": {
          "type": "integer",
          "description": "Request timeout in seconds",
          "default": 30,
          "minimum": 1,
          "maximum": 60
        },
        "headers": {
          "type": "object",
          "description": "Custom HTTP headers",
          "additionalProperties": {
            "type": "string"
          }
        }
      },
      "additionalProperties": false
    }
  }
}
```

## Examples

### Minimal Configuration

```json
{
  "endpoints": [
    {
      "name": "Production API",
      "url": "https://api.example.com/health"
    }
  ]
}
```

### Full Configuration

```json
{
  "endpoints": [
    {
      "name": "Production API",
      "url": "https://api.example.com/health",
      "interval_minutes": 5,
      "timeout_seconds": 10,
      "headers": {
        "X-Health-Check": "monitoring-service",
        "Authorization": "Bearer secret-token"
      }
    },
    {
      "name": "Marketing Website",
      "url": "https://www.example.com/",
      "interval_minutes": 15,
      "timeout_seconds": 30
    },
    {
      "name": "Staging API",
      "url": "https://staging-api.example.com/health",
      "interval_minutes": 30,
      "headers": {
        "X-Environment": "staging"
      }
    }
  ]
}
```

### With Authentication Header

```json
{
  "endpoints": [
    {
      "name": "Protected Endpoint",
      "url": "https://api.example.com/internal/health",
      "headers": {
        "Authorization": "Bearer eyJhbGciOiJIUzI1NiIs...",
        "X-API-Key": "your-api-key"
      }
    }
  ]
}
```

## Validation Rules

1. **URL must be HTTPS** - HTTP URLs are not supported for security reasons
2. **Name must be unique** - Each endpoint should have a distinct name for clear alerting
3. **Interval minimum is 1 minute** - More frequent checks are not supported
4. **Timeout cannot exceed 60 seconds** - Lambda has execution limits
5. **Headers are optional** - Only include if the endpoint requires authentication or custom headers

## Security Considerations

- **Sensitive headers**: Authorization tokens and API keys in the `headers` field are stored in plaintext in S3. Ensure the S3 bucket has appropriate access controls.
- **Future enhancement**: Consider using AWS Secrets Manager for sensitive header values.
- **Bucket access**: The S3 bucket policy restricts access to the Lambda execution role only.

## File Location

The configuration file is stored at:

```
s3://cheap-uptime-monitor-config-bucket/config.json
```

To update the configuration:

```bash
aws s3 cp config.json s3://cheap-uptime-monitor-config-bucket/config.json
```

Changes take effect on the next scheduled Lambda execution.
