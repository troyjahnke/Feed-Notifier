terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "4.20.1"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

resource "aws_secretsmanager_secret" "notification_secret" {
  name = "notificationFinal"
}

resource "aws_secretsmanager_secret_version" "shoutrrr_secret_version" {
  secret_id     = aws_secretsmanager_secret.notification_secret.arn
  secret_string = jsonencode(var.shoutrrr_url)
}

resource "aws_dynamodb_table" "feed_table" {
  name           = "feeds"
  hash_key       = "name"
  read_capacity  = 10
  write_capacity = 10

  attribute {
    name = "name"
    type = "S"
  }
}

resource "aws_dynamodb_table_item" "item" {
  hash_key   = aws_dynamodb_table.feed_table.hash_key
  table_name = aws_dynamodb_table.feed_table.name
  for_each   = var.feeds
  item       = jsonencode(
    {
      "name" : { "S" : "${each.key}" },
      "url" : { "S" : "${each.value}" }
    })
}

data "aws_iam_policy_document" "inlinePolicy" {
  statement {
    effect = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      identifiers = ["lambda.amazonaws.com"]
      type        = "Service"
    }
  }
}

data "aws_iam_policy_document" "dynamoDB" {
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

data "aws_iam_policy_document" "secretManager" {
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

data "aws_iam_policy_document" "cloudWatch" {
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

resource "aws_iam_policy" "cloudwatch" {
  policy = data.aws_iam_policy_document.cloudWatch.json
}

resource "aws_iam_policy" "dynamoDB" {
  policy = data.aws_iam_policy_document.dynamoDB.json
}

resource "aws_iam_policy" "secretsManager" {
  policy = data.aws_iam_policy_document.secretManager.json
}

resource "aws_iam_role_policy_attachment" "cloudWatch" {
  policy_arn = aws_iam_policy.cloudwatch.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role_policy_attachment" "dynamoDB" {
  policy_arn = aws_iam_policy.dynamoDB.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role_policy_attachment" "secretsManager" {
  policy_arn = aws_iam_policy.secretsManager.arn
  role       = aws_iam_role.feedNotifier.name
}

resource "aws_iam_role" "feedNotifier" {
  name               = "feedNotifier"
  assume_role_policy = data.aws_iam_policy_document.inlinePolicy.json

  managed_policy_arns = [aws_iam_policy.cloudwatch.arn,
    aws_iam_policy.dynamoDB.arn,
    aws_iam_policy.secretsManager.arn]
}

resource "aws_lambda_function" "feed_notifier" {
  function_name = "FeedNotifier"
  role          = aws_iam_role.feedNotifier.arn
  runtime       = "go1.x"
  handler       = "FeedNotifier"
  filename      = "C:/Users/troy/dev/checkout/gitlab/feednotifier/FeedNotifier.zip"
  timeout = var.feed_notifier_timeout
}

resource "aws_cloudwatch_event_rule" "feed_notifier" {
  name = "FeedNotifierScheduler"
  schedule_expression = "rate(3 hours)"
}

resource "aws_cloudwatch_event_target" "feed_notifier" {
  arn  = aws_lambda_function.feed_notifier.arn
  rule = aws_cloudwatch_event_rule.feed_notifier.name
}

resource "aws_cloudwatch_log_group" "feed_notifier" {
  name = "/aws/lambda/${aws_lambda_function.feed_notifier.function_name}"
  retention_in_days = 1
}