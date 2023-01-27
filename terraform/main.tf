terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

resource "aws_secretsmanager_secret" "notification_secret" {
  name = var.notification_secret_name
}

resource "aws_secretsmanager_secret_version" "shoutrrr_secret_version" {
  secret_id     = aws_secretsmanager_secret.notification_secret.arn
  secret_string = jsonencode(var.shoutrrr_url)
}

resource "aws_dynamodb_table" "feed_table" {
  name           = var.table_name
  hash_key       = "name"
  read_capacity  = 10
  write_capacity = 10

  attribute {
    name = "name"
    type = "S"
  }
}

data "aws_iam_policy_document" "feed_notifier_inline_policy" {
  statement {
    effect = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      identifiers = ["lambda.amazonaws.com"]
      type        = "Service"
    }
  }
}

data "aws_iam_policy_document" "feed_db" {
  statement {
    effect = "Allow"
    actions = [
      "dynamodb:Get*",
      "dynamodb:Query",
      "dynamodb:Scan",
      "dynamodb:Update*",
      "dynamodb:PutItem"
    ]
    resources = [aws_dynamodb_table.feed_table.arn]
  }
}

data "aws_iam_policy_document" "notification_secret" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue"
    ]
    resources = [
      aws_secretsmanager_secret.notification_secret.arn,
      aws_secretsmanager_secret_version.shoutrrr_secret_version.arn
    ]
  }
}

data "aws_iam_policy_document" "cloud_watch" {
  statement {
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }
}

resource "aws_iam_policy" "feed_function_cloud_watch" {
  policy = data.aws_iam_policy_document.cloud_watch.json
}

resource "aws_iam_policy" "feed_db" {
  policy = data.aws_iam_policy_document.feed_db.json
}

resource "aws_iam_policy" "notification_secret" {
  policy = data.aws_iam_policy_document.notification_secret.json
}

resource "aws_iam_role_policy_attachment" "feed_function_cloud_watch" {
  policy_arn = aws_iam_policy.feed_function_cloud_watch.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role_policy_attachment" "feed_db" {
  policy_arn = aws_iam_policy.feed_db.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role_policy_attachment" "notification_secret" {
  policy_arn = aws_iam_policy.notification_secret.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role" "feedNotifier" {
  name               = "feedNotifier"
  assume_role_policy = data.aws_iam_policy_document.feed_notifier_inline_policy.json

  managed_policy_arns = [aws_iam_policy.feed_function_cloud_watch.arn,
    aws_iam_policy.feed_db.arn,
    aws_iam_policy.notification_secret.arn]
}

resource "aws_lambda_function" "feed_notifier" {
  function_name = "FeedNotifier"
  role          = aws_iam_role.feedNotifier.arn
  runtime       = "go1.x"
  handler       = "FeedNotifier"
  filename      = var.feed_notifier_zip
  timeout = var.feed_notifier_timeout
  environment {
    variables = {
      TABLE_NAME=var.table_name
      SECRET_NAME=var.notification_secret_name
    }
  }
}

resource "aws_cloudwatch_log_group" "feed_notifier" {
  name = "/aws/lambda/${aws_lambda_function.feed_notifier.function_name}"
  retention_in_days = 1
}

// Event Bridge

resource "aws_cloudwatch_event_rule" "scheduler" {
  name = "FeedNotifierScheduler"
  schedule_expression = "rate(3 hours)"
}

resource "aws_cloudwatch_event_target" "scheduler" {
  arn  = aws_lambda_function.feed_notifier.arn
  rule = aws_cloudwatch_event_rule.scheduler.name
}

resource "aws_lambda_permission" "scheduler" {
  action        = "lambda:invokeFunction"
  function_name = aws_lambda_function.feed_notifier.function_name
  principal     = "events.amazonaws.com"
  source_arn = aws_cloudwatch_event_rule.scheduler.arn
}

terraform {
  backend "s3" {
    bucket = "aerzus-terraform"
    key = "feednotifier.tfstate"
    region = "us-east-1"
  }
}