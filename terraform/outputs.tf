output "lambda_function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.monitor.function_name
}

output "lambda_function_arn" {
  description = "ARN of the Lambda function"
  value       = aws_lambda_function.monitor.arn
}

output "config_bucket_name" {
  description = "Name of the S3 bucket for configuration"
  value       = aws_s3_bucket.config.id
}

output "config_bucket_arn" {
  description = "ARN of the S3 bucket for configuration"
  value       = aws_s3_bucket.config.arn
}

output "sns_topic_arn" {
  description = "ARN of the SNS topic for alerts"
  value       = aws_sns_topic.alerts.arn
}

output "cloudwatch_log_group" {
  description = "CloudWatch Log Group for Lambda logs"
  value       = aws_cloudwatch_log_group.lambda.name
}

output "config_upload_command" {
  description = "Command to upload configuration to S3"
  value       = "aws s3 cp config/config.json s3://${aws_s3_bucket.config.id}/${var.config_key}"
}
