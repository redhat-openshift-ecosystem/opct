variable "subnet_id" {}
variable "instance_type" {}
variable "ami_id" {}
variable "cluster_member_sg_id" {}
variable "cluster_instance_profile_name"  {}
variable "key_name"  {}
variable "instance_kind" {
  default = "instance"
}
variable "extra_user_data" {}