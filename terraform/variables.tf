variable "alert_email" {
  description = "Email address to receive alerts (leave empty to skip email subscription)"
  type        = string
  default     = ""
}

variable "config_key" {
  description = "S3 key for the configuration file"
  type        = string
  default     = "config.json"
}

variable "health_key" {
  description = "S3 key for the known health tracking file"
  type        = string
  default     = "known-health.json"
}

variable "check_interval_minutes" {
  description = "Default interval in minutes between health checks"
  type        = number
  default     = 15
}

variable "github_repo" {
  description = "GitHub repository in owner/repo format for OIDC trust policy"
  type        = string
}
