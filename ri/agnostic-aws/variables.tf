variable "aws_region" {
  default = "us-west-2"
}

variable "instance_type" {
  default = "t3.xlarge"
}

variable "architecture" {
  default = "x86_64"
}

// https://access.redhat.com/solutions/15356
variable "ami_mapping" {
  type = map(map(string))
  default = {
    "us-west-2" = {
      "x86_64" = "ami-08970fb2e5767e3b8"
      "arm64"  = "ami-0bb199dd39edd7d71"
    }
  }
}


variable "base_domain" {
  default = "devcluster.openshift.com"
}

variable "cluster_name" {
  default = "opct20230903"
}
