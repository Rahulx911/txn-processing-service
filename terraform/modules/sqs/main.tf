variable "queue_name" {
  type = string
}

variable "visibility_timeout" {
  type    = number
  default = 300
}

variable "message_retention_seconds" {
  type    = number
  default = 1209600
}

variable "max_receive_count" {
  type    = number
  default = 3
}

variable "enable_dlq" {
  type    = bool
  default = true
}

variable "dlq_name" {
  type    = string
  default = ""
}

variable "tags" {
  type    = map(string)
  default = {}
}

resource "aws_sqs_queue" "dlq" {
  count = var.enable_dlq ? 1 : 0

  name                      = var.dlq_name
  message_retention_seconds = 1209600 # 14 days

  tags = merge(var.tags, { Type = "dead-letter-queue" })
}

resource "aws_sqs_queue" "this" {
  name                       = var.queue_name
  visibility_timeout_seconds = var.visibility_timeout
  message_retention_seconds  = var.message_retention_seconds

  redrive_policy = var.enable_dlq ? jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[0].arn
    maxReceiveCount     = var.max_receive_count
  }) : null

  tags = var.tags
}

output "queue_url" {
  value = aws_sqs_queue.this.url
}

output "queue_arn" {
  value = aws_sqs_queue.this.arn
}

output "dlq_url" {
  value = var.enable_dlq ? aws_sqs_queue.dlq[0].url : ""
}
