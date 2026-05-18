resource "aws_s3_bucket" "artifacts" {
  bucket = "${local.resource_prefix}-${data.aws_caller_identity.current.account_id}-artifacts"

  tags = {
    Boundary           = "artifact-storage"
    DataClassification = "customer-artifact"
  }
}

resource "aws_s3_bucket_public_access_block" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_versioning" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id

  rule {
    id     = "abort-incomplete-multipart-uploads"
    status = "Enabled"

    filter {
      prefix = ""
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }
}

resource "aws_iam_role_policy" "control_plane_artifacts" {
  name = "${local.resource_prefix}-control-plane-artifacts"
  role = aws_iam_role.control_plane_runtime.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:AbortMultipartUpload",
          "s3:GetObject",
          "s3:GetObjectAttributes"
        ]
        Resource = "${aws_s3_bucket.artifacts.arn}/artifacts/*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucketMultipartUploads"
        ]
        Resource = aws_s3_bucket.artifacts.arn
      }
    ]
  })
}
