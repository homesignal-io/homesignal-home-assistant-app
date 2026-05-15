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
