variable "topic_name" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}

resource "aws_sns_topic" "this" {
  name = var.topic_name
  tags = var.tags
}

output "topic_arn" {
  value = aws_sns_topic.this.arn
}

output "topic_name" {
  value = aws_sns_topic.this.name
}
