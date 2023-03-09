variable "aws_region" {
  default = "us-west-2"
}

variable "instance_type" {
  default = "r6i.large"
}

variable "architecture" {
  default = "x86_64"
}

variable "base_domain" {
  default = "devcluster.openshift.com"
}

variable "cluster_name" {
  default = "opct20230903"
}
