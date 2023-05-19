variable "aws_region" {}

variable "vpc_cidr_block" {
  default = "10.0.0.0/16"
}

variable "public_az_1" {
  default = "us-west-2a"
}

variable "public_az_2" {
  default = "us-west-2b"
}

variable "public_az_3" {
  default = "us-west-2c"
}

variable "private_az_1" {
  default = "us-west-2a"
}

variable "private_az_2" {
  default = "us-west-2b"
}

variable "private_az_3" {
  default = "us-west-2c"
}

variable "public_subnet_cidr_blocks" {
  type = list(string)
  default = ["10.0.1.0/24", "10.0.3.0/24", "10.0.5.0/24"]
}

variable "private_subnet_cidr_blocks" {
  type = list(string)
  default = ["10.0.2.0/24", "10.0.4.0/24", "10.0.6.0/24"]
}
