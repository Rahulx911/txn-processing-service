#!/bin/bash
set -euo pipefail

echo "Initialising LocalStack AWS resources..."

REGION="ap-south-1"
ENDPOINT="http://localhost:4566"

# DynamoDB: Transactions table
awslocal dynamodb create-table \
  --table-name transactions \
  --attribute-definitions AttributeName=PK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "$REGION"

# DynamoDB: Idempotency keys table
awslocal dynamodb create-table \
  --table-name transactions-idempotency \
  --attribute-definitions AttributeName=PK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "$REGION"

# Enable TTL on idempotency table
awslocal dynamodb update-time-to-live \
  --table-name transactions-idempotency \
  --time-to-live-specification Enabled=true,AttributeName=ttl \
  --region "$REGION"

# SQS: Processing queue
awslocal sqs create-queue \
  --queue-name txn-processing-dlq \
  --region "$REGION"

DLQ_ARN=$(awslocal sqs get-queue-attributes \
  --queue-url http://localhost:4566/000000000000/txn-processing-dlq \
  --attribute-names QueueArn \
  --query 'Attributes.QueueArn' \
  --output text \
  --region "$REGION")

awslocal sqs create-queue \
  --queue-name txn-processing \
  --attributes "{\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}" \
  --region "$REGION"

# SNS: Notification topic
awslocal sns create-topic \
  --name txn-notifications \
  --region "$REGION"

# EventBridge: Custom event bus
awslocal events create-event-bus \
  --name txn-service-events \
  --region "$REGION"

echo "LocalStack initialisation complete!"
