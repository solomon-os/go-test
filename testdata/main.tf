# Sample Terraform Configuration for EC2 Instances
# This file demonstrates the expected Terraform configuration format

terraform {
  required_version = ">= 1.0.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

# Web Server Instance
resource "aws_instance" "web_server" {
  ami                    = "ami-0123456789abcdef0"
  instance_type          = "t2.micro"
  key_name               = "production-key"
  subnet_id              = "subnet-12345678"
  vpc_security_group_ids = ["sg-0123456789abcdef0", "sg-0987654321fedcba0"]

  ebs_optimized = false
  monitoring    = true

  iam_instance_profile = "WebServerRole"

  root_block_device {
    volume_size           = 50
    volume_type           = "gp3"
    delete_on_termination = true
    encrypted             = true
    iops                  = 3000
    throughput            = 125
  }

  tags = {
    Name        = "web-server-01"
    Environment = "production"
    Team        = "platform"
  }
}

# API Server Instance
resource "aws_instance" "api_server" {
  ami                    = "ami-0123456789abcdef0"
  instance_type          = "t3.medium"
  key_name               = "production-key"
  subnet_id              = "subnet-87654321"
  vpc_security_group_ids = ["sg-1111111111111111", "sg-2222222222222222"]

  ebs_optimized = true
  monitoring    = true

  iam_instance_profile = "APIServerRole"

  root_block_device {
    volume_size           = 100
    volume_type           = "gp3"
    delete_on_termination = true
    encrypted             = true
    iops                  = 4000
    throughput            = 250
  }

  tags = {
    Name        = "api-server-01"
    Environment = "production"
    Team        = "backend"
  }
}

# Database Server Instance
resource "aws_instance" "database_server" {
  ami                    = "ami-0fedcba9876543210"
  instance_type          = "r5.large"
  key_name               = "database-key"
  subnet_id              = "subnet-11223344"
  vpc_security_group_ids = ["sg-3333333333333333"]

  ebs_optimized = true
  monitoring    = true

  iam_instance_profile = "DatabaseRole"

  root_block_device {
    volume_size           = 500
    volume_type           = "io2"
    delete_on_termination = false
    encrypted             = true
    iops                  = 10000
  }

  tags = {
    Name        = "database-server-01"
    Environment = "production"
    Team        = "data"
  }
}
