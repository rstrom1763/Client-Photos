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

# Networking
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

resource "aws_security_group" "web_sg" {
  name        = "web-server-sg"
  description = "Allow HTTP, HTTPS and SSH"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

}

# IAM Role for EC2
resource "aws_iam_role" "web_role" {
  name = "web-server-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ssm_policy" {
  role       = aws_iam_role.web_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMFullAccess"
}

resource "aws_iam_role_policy_attachment" "s3_policy" {
  role       = aws_iam_role.web_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}

resource "aws_iam_role_policy_attachment" "dynamodb_policy" {
  role       = aws_iam_role.web_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"
}

resource "aws_iam_instance_profile" "web_profile" {
  name = "web-server-profile"
  role = aws_iam_role.web_role.name
}

# EC2 Instance
data "aws_ami" "amazon_linux_2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023*-x86_64"]
  }
}

resource "aws_instance" "web_server" {
  ami           = data.aws_ami.amazon_linux_2023.id
  instance_type = "t3a.small"
  
  subnet_id              = data.aws_subnets.default.ids[0]
  vpc_security_group_ids = [aws_security_group.web_sg.id]
  iam_instance_profile   = aws_iam_instance_profile.web_profile.name

  user_data = <<-EOF
              #!/bin/bash
              # Install dependencies
              sudo yum update -y
              sudo yum install -y git golang

              # Clone repo
              cd /home/ec2-user
              git clone https://github.com/rstrom1763/Client-Photos.git;
              chown -R ec2-user:ec2-user Client-Photos;
              cd Client-Photos;

              # Configure Git to trust the repository directory (avoids VCS status errors)
              git config --global --add safe.directory /home/ec2-user/Client-Photos;

              # Set Go environment variables
              export HOME=/home/ec2-user
              export GOPATH=/home/ec2-user/go
              export GOMODCACHE=/home/ec2-user/go/pkg/mod
              export GOCACHE=/home/ec2-user/.cache/go-build
              export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

              # Ensure cache directories exist with proper ownership
              mkdir -p /home/ec2-user/.cache/go-build
              chown -R ec2-user:ec2-user /home/ec2-user/.cache
              chown -R ec2-user:ec2-user /home/ec2-user/go

              # Create .env file for the application
              echo '
              PORT="443"
              MAXROUTINES="10"
              PROTOCOL="https"
              MAXPICS="20"
              MINUTES="15"
              DEBUG="false"
              REGION="${data.aws_region.current.name}"
              ' > .env;

              # Ensure dependencies are tidy before build
              go mod tidy

              # Build the application
              cd api
              go build -buildvcs=false .
              strip ./api
              sudo setcap CAP_NET_BIND_SERVICE=+eip /home/ec2-user/Client-Photos/api/api;

              # Setup systemd service to run the app
              echo '
              [Unit]
              Description=Client Photos Web Server
              After=network.target

              [Service]
              Type=simple
              User=root
              WorkingDirectory=/home/ec2-user/Client-Photos/api
              ExecStart=/home/ec2-user/Client-Photos/api/api
              Restart=always
              Environment=REGION=${data.aws_region.current.name}
              Environment=GOPATH=/home/ec2-user/go
              Environment=GOMODCACHE=/home/ec2-user/go/pkg/mod
              Environment=GOCACHE=/home/ec2-user/.cache/go-build
              Environment=HOME=/home/ec2-user

              [Install]
              WantedBy=multi-user.target
              ' > client-photos.service
              sudo mv client-photos.service /etc/systemd/system/client-photos.service

              sudo systemctl daemon-reload
              sudo systemctl enable client-photos
              sudo systemctl start client-photos
              EOF

  tags = {
    Name = "ClientPhotosWebServer"
  }
}

output "web_server_public_ip" {
  value = aws_instance.web_server.public_ip
}

output "web_server_url" {
  value = "https://${aws_instance.web_server.public_ip}"
}