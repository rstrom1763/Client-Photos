provider "aws" {
  region = "us-east-2"
}

resource aws_dynamodb_table "request_logs" {

  name = "request_logs"
  billing_mode = "PAY_PER_REQUEST"
  deletion_protection_enabled = true
  hash_key = "request_id"
  range_key = "timestamp"

  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  attribute {
    name = "request_id"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "N"
  }

  attribute {
    name = "method"
    type = "S"
  }

  attribute {
    name = "status"
    type = "N"
  }

  attribute {
    name = "remote_ip"
    type = "S"
  }

  replica {
    region_name = "eu-central-1"
    consistency_mode = "EVENTUAL"
  }

  global_secondary_index {

    key_schema {
      attribute_name = "method"
      key_type = "HASH"
    }

    key_schema {
      attribute_name = "status"
      key_type = "RANGE"
    }

    key_schema {
      attribute_name = "timestamp"
      key_type = "RANGE"
    }

    name            = "method_gsi"
    projection_type = "ALL"
  }

  global_secondary_index {

    key_schema {
      attribute_name = "remote_ip"
      key_type = "HASH"
    }

    key_schema {
      attribute_name = "timestamp"
      key_type = "RANGE"
    }

    name            = "remote_ip_gsi"
    projection_type = "ALL"
  }

}

resource "aws_dynamodb_table" "user_sessions" {
  name           = "user_sessions"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "username"
  range_key      = "token"
  deletion_protection_enabled = true

  attribute {
    name = "username"
    type = "S"
  }

  attribute {
    name = "token"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }
}

resource "aws_dynamodb_table" "photo-clients" {
  name           = "photo-clients"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "username"
  deletion_protection_enabled = true

  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  attribute {
    name = "username"
    type = "S"
  }

  replica {
    region_name = "eu-west-3"
    consistency_mode = "EVENTUAL"
  }

  replica {
    region_name      = "us-west-2"
    consistency_mode = "EVENTUAL"
  }

}

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

resource "random_id" "bucket_id" {
  byte_length = 4
}

resource "aws_s3_bucket" "photos_bucket" {
  bucket = "client-photos-${data.aws_region.current.name}-${data.aws_caller_identity.current.account_id}-${random_id.bucket_id.hex}"
}

resource "aws_s3_bucket_public_access_block" "photos_public_access" {
  bucket = aws_s3_bucket.photos_bucket.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "photos_versioning" {
  bucket = aws_s3_bucket.photos_bucket.id
  versioning_configuration {
    status = "Disabled"
  }
}

# SSM Parameters to store configuration
resource "aws_ssm_parameter" "region" {
  name  = "/app/region"
  type  = "String"
  value = "us-east-2"
}

resource "aws_ssm_parameter" "bucket" {
  name  = "/app/bucket"
  type  = "String"
  value = aws_s3_bucket.photos_bucket.bucket
}

resource "aws_ssm_parameter" "tablename" {
  name  = "/app/tablename"
  type  = "String"
  value = aws_dynamodb_table.photo-clients.name
}

resource "aws_ssm_parameter" "session_tablename" {
  name  = "/app/session_tablename"
  type  = "String"
  value = aws_dynamodb_table.user_sessions.name
}

resource "aws_ssm_parameter" "log_tablename" {
  name  = "/app/log_tablename"
  type  = "String"
  value = aws_dynamodb_table.request_logs.name
}