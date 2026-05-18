locals {
  cognito_issuer        = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.portal_users.id}"
  cognito_domain_prefix = var.cognito_domain_prefix != "" ? var.cognito_domain_prefix : "${local.resource_prefix}-${data.aws_caller_identity.current.account_id}"
}

resource "aws_cognito_user_pool" "portal_users" {
  name = "${local.resource_prefix}-portal-users"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  mfa_configuration        = "OFF"

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 7
  }

  tags = merge(local.tags, {
    Boundary = "auth"
  })
}

resource "aws_cognito_user_pool_client" "portal" {
  name         = "${local.resource_prefix}-portal"
  user_pool_id = aws_cognito_user_pool.portal_users.id

  generate_secret                      = false
  prevent_user_existence_errors        = "ENABLED"
  supported_identity_providers         = ["COGNITO"]
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid", "profile"]

  callback_urls = var.portal_callback_urls
  logout_urls   = var.portal_logout_urls

  explicit_auth_flows = [
    "ALLOW_REFRESH_TOKEN_AUTH",
    "ALLOW_USER_SRP_AUTH",
  ]

  access_token_validity  = 1
  id_token_validity      = 1
  refresh_token_validity = 30

  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }
}

resource "aws_cognito_user_pool_domain" "portal" {
  domain       = local.cognito_domain_prefix
  user_pool_id = aws_cognito_user_pool.portal_users.id
}
