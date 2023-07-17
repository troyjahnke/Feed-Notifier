variable "aws_region" {
  default = "us-east-1"
}
variable "shoutrrr_url" {}
variable "feed_notifier_timeout" {
  default = 10
  type = number
}
variable "table_name" {
  default = "feeds"
  type = string
}
variable "notification_secret_name" {
  default = "notification-url"
}
variable "feed_notifier_zip"  {
  default = "../FeedNotifier.zip"
}
