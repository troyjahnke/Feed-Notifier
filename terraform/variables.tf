variable "aws_region" {
  default = "us-east-1"
}
variable "shoutrrr_url" {}
variable "feeds"{
  type = map(string)
}
variable "feed_notifier_timeout" {
  default = 10
  type = number
}