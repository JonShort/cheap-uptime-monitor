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

variable "check_interval_minutes" {
  description = "Default interval in minutes between health checks"
  type        = number
  default     = 15
}
