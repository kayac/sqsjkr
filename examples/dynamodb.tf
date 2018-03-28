resource "aws_dynamodb_table" "cron" {
  name           = "Cron"
  read_capacity  = 5
  write_capacity = 5
  hash_key       = "Id"
  range_key      = "Type"

  attribute {
    name = "Id"
    type = "S"
  }

  attribute {
    name = "Type"
    type = "S"
  }

  attribute {
    name = "Expired"
    type = "N"
  }

  global_secondary_index {
    name               = "TypeExpiredIndex"
    hash_key           = "Type"
    range_key          = "Expired"
    write_capacity     = 5
    read_capacity      = 5
    projection_type    = "ALL"
  }
}
