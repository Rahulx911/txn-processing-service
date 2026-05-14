variable "table_name" {
  type = string
}

variable "hash_key" {
  type    = string
  default = "PK"
}

variable "billing_mode" {
  type    = string
  default = "PAY_PER_REQUEST"
}

variable "attributes" {
  type = list(object({
    name = string
    type = string
  }))
}

variable "ttl_enabled" {
  type    = bool
  default = false
}

variable "ttl_attribute" {
  type    = string
  default = "ttl"
}

variable "point_in_time_recovery" {
  type    = bool
  default = true
}

variable "tags" {
  type    = map(string)
  default = {}
}

resource "aws_dynamodb_table" "this" {
  name         = var.table_name
  billing_mode = var.billing_mode
  hash_key     = var.hash_key

  dynamic "attribute" {
    for_each = var.attributes
    content {
      name = attribute.value.name
      type = attribute.value.type
    }
  }

  dynamic "ttl" {
    for_each = var.ttl_enabled ? [1] : []
    content {
      attribute_name = var.ttl_attribute
      enabled        = true
    }
  }

  point_in_time_recovery {
    enabled = var.point_in_time_recovery
  }

  tags = var.tags
}

output "table_name" {
  value = aws_dynamodb_table.this.name
}

output "table_arn" {
  value = aws_dynamodb_table.this.arn
}
