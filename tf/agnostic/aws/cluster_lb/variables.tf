variable "vpc_id" {}

variable "subnet_ids" {
  type = list(string)
}

variable "control_plane_instances_ids" {
}
