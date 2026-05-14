terraform {
  required_version = ">= 1.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.30"
    }
  }

  backend "s3" {
    bucket = "txn-service-terraform-state"
    key    = "infrastructure/terraform.tfstate"
    region = "ap-south-1"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "txn-processing-service"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# --- DynamoDB: Transactions Table ---
module "transactions_table" {
  source = "./modules/dynamodb"

  table_name   = "${var.project_name}-${var.environment}"
  hash_key     = "PK"
  billing_mode = "PAY_PER_REQUEST"

  attributes = [
    { name = "PK", type = "S" }
  ]

  ttl_enabled        = false
  point_in_time_recovery = true

  tags = {
    Component = "transactions"
  }
}

# --- DynamoDB: Idempotency Keys Table ---
module "idempotency_table" {
  source = "./modules/dynamodb"

  table_name   = "${var.project_name}-${var.environment}-idempotency"
  hash_key     = "PK"
  billing_mode = "PAY_PER_REQUEST"

  attributes = [
    { name = "PK", type = "S" }
  ]

  ttl_enabled       = true
  ttl_attribute     = "ttl"
  point_in_time_recovery = false

  tags = {
    Component = "idempotency"
  }
}

# --- SQS: Transaction Processing Queue ---
module "transaction_queue" {
  source = "./modules/sqs"

  queue_name                = "${var.project_name}-${var.environment}-processing"
  visibility_timeout        = 300
  message_retention_seconds = 1209600 # 14 days
  max_receive_count         = 3

  enable_dlq = true
  dlq_name   = "${var.project_name}-${var.environment}-processing-dlq"

  tags = {
    Component = "async-processing"
  }
}

# --- SNS: Transaction Notifications ---
module "transaction_notifications" {
  source = "./modules/sns"

  topic_name = "${var.project_name}-${var.environment}-notifications"

  tags = {
    Component = "notifications"
  }
}

# --- EventBridge: Domain Events ---
module "transaction_events" {
  source = "./modules/eventbridge"

  bus_name    = "${var.project_name}-${var.environment}-events"
  environment = var.environment

  rules = {
    transaction_settled = {
      description   = "Triggers when a transaction is settled"
      event_pattern = jsonencode({
        source      = ["txn-processing-service"]
        detail-type = ["TransactionSettled"]
      })
      target_arn = module.transaction_queue.queue_arn
    }
    transaction_failed = {
      description   = "Triggers when a transaction fails"
      event_pattern = jsonencode({
        source      = ["txn-processing-service"]
        detail-type = ["TransactionFailed"]
      })
      target_arn = module.transaction_notifications.topic_arn
    }
  }

  tags = {
    Component = "event-routing"
  }
}
