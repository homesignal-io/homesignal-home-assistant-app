output "aws_account_id" {
  description = "AWS account ID resolved by the staging deployment."
  value       = data.aws_caller_identity.current.account_id
}

output "aws_region" {
  description = "AWS region used by the staging deployment."
  value       = var.aws_region
}

output "control_plane_log_group" {
  description = "CloudWatch log group for the staging control-plane Lambda."
  value       = aws_cloudwatch_log_group.control_plane.name
}

output "resource_prefix" {
  description = "Resource name prefix used by staging infrastructure."
  value       = local.resource_prefix
}

output "staging_base_url" {
  description = "Base URL for the staging HTTP API."
  value       = aws_apigatewayv2_api.public.api_endpoint
}

output "artifact_bucket_name" {
  description = "Private S3 bucket for staging brokered artifacts."
  value       = aws_s3_bucket.artifacts.bucket
}

output "artifact_bucket_arn" {
  description = "ARN of the private S3 bucket for staging brokered artifacts."
  value       = aws_s3_bucket.artifacts.arn
}

output "telemetry_ingest_ecr_repository_url" {
  description = "ECR repository URL for telemetry-ingest images."
  value       = aws_ecr_repository.telemetry_ingest.repository_url
}

output "telemetry_ingest_cluster_name" {
  description = "ECS cluster that runs telemetry-ingest."
  value       = aws_ecs_cluster.runtime.name
}

output "telemetry_ingest_service_name" {
  description = "ECS service name for telemetry-ingest."
  value       = aws_ecs_service.telemetry_ingest.name
}

output "telemetry_ingest_log_group" {
  description = "CloudWatch log group for telemetry-ingest."
  value       = aws_cloudwatch_log_group.telemetry_ingest.name
}

output "iot_device_policy_name" {
  description = "AWS IoT device policy name for claimed runtime devices."
  value       = aws_iot_policy.device.name
}

output "iot_lifecycle_rule_name" {
  description = "AWS IoT lifecycle topic rule name."
  value       = aws_iot_topic_rule.lifecycle.name
}

output "iot_lifecycle_log_group" {
  description = "CloudWatch log group for AWS IoT lifecycle events."
  value       = aws_cloudwatch_log_group.iot_lifecycle.name
}

output "database_url_secret_name" {
  description = "Secrets Manager name for the staging PostgreSQL connection URL."
  value       = aws_secretsmanager_secret.database_url.name
}

output "database_url_secret_arn" {
  description = "Secrets Manager ARN for the staging PostgreSQL connection URL."
  value       = aws_secretsmanager_secret.database_url.arn
}

output "database_provider_parameter_name" {
  description = "SSM parameter that records the staging PostgreSQL provider."
  value       = aws_ssm_parameter.database_provider.name
}

output "database_region_parameter_name" {
  description = "SSM parameter that records the expected staging PostgreSQL region."
  value       = aws_ssm_parameter.database_region.name
}

output "cognito_user_pool_id" {
  description = "Cognito user pool ID for staging human portal/API authentication."
  value       = aws_cognito_user_pool.portal_users.id
}

output "cognito_user_pool_client_id" {
  description = "Cognito app client ID for staging portal authentication."
  value       = aws_cognito_user_pool_client.portal.id
}

output "cognito_issuer" {
  description = "Cognito JWT issuer expected by the control-plane runtime."
  value       = local.cognito_issuer
}

output "cognito_hosted_ui_domain" {
  description = "Cognito hosted UI domain prefix for staging portal authentication."
  value       = aws_cognito_user_pool_domain.portal.domain
}
