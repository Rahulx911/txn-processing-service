output "transactions_table_name" {
  description = "DynamoDB transactions table name"
  value       = module.transactions_table.table_name
}

output "idempotency_table_name" {
  description = "DynamoDB idempotency table name"
  value       = module.idempotency_table.table_name
}

output "processing_queue_url" {
  description = "SQS processing queue URL"
  value       = module.transaction_queue.queue_url
}

output "notification_topic_arn" {
  description = "SNS notification topic ARN"
  value       = module.transaction_notifications.topic_arn
}

output "event_bus_name" {
  description = "EventBridge event bus name"
  value       = module.transaction_events.bus_name
}
