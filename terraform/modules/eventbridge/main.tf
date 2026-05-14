variable "bus_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "rules" {
  type = map(object({
    description   = string
    event_pattern = string
    target_arn    = string
  }))
  default = {}
}

variable "tags" {
  type    = map(string)
  default = {}
}

resource "aws_cloudwatch_event_bus" "this" {
  name = var.bus_name
  tags = var.tags
}

resource "aws_cloudwatch_event_rule" "rules" {
  for_each = var.rules

  name           = "${var.bus_name}-${each.key}"
  description    = each.value.description
  event_bus_name = aws_cloudwatch_event_bus.this.name
  event_pattern  = each.value.event_pattern

  tags = var.tags
}

resource "aws_cloudwatch_event_target" "targets" {
  for_each = var.rules

  rule           = aws_cloudwatch_event_rule.rules[each.key].name
  event_bus_name = aws_cloudwatch_event_bus.this.name
  arn            = each.value.target_arn
  target_id      = each.key
}

output "bus_name" {
  value = aws_cloudwatch_event_bus.this.name
}

output "bus_arn" {
  value = aws_cloudwatch_event_bus.this.arn
}
