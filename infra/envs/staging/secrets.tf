locals {
  database_url_secret_name      = "/homesignal/${local.environment}/platform/database_url"
  database_provider_config_name = "/homesignal/${local.environment}/platform/config/database_provider"
  database_region_config_name   = "/homesignal/${local.environment}/platform/config/database_region"
  database_workload_config_name = "/homesignal/${local.environment}/platform/config/database_workload"
}

resource "aws_secretsmanager_secret" "database_url" {
  name                    = local.database_url_secret_name
  description             = "HomeSignal staging PostgreSQL connection URL for migration and runtime wiring."
  recovery_window_in_days = 0

  tags = {
    Boundary           = "platform"
    DataClassification = "secret"
  }
}

resource "aws_ssm_parameter" "database_provider" {
  name        = local.database_provider_config_name
  description = "Provider hosting the HomeSignal staging PostgreSQL database."
  type        = "String"
  value       = "neon"

  tags = {
    Boundary = "platform"
  }
}

resource "aws_ssm_parameter" "database_region" {
  name        = local.database_region_config_name
  description = "Expected deployment region for the HomeSignal staging PostgreSQL database."
  type        = "String"
  value       = "us-east-1"

  tags = {
    Boundary = "platform"
  }
}

resource "aws_ssm_parameter" "database_workload" {
  name        = local.database_workload_config_name
  description = "Current HomeSignal database workload boundary."
  type        = "String"
  value       = "staging-v0-platform"

  tags = {
    Boundary = "platform"
  }
}
