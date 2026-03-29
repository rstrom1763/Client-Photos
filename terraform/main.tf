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