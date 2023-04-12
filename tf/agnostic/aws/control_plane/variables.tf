variable "subnet_ids" {
    type = list(string)
}
variable "instance_type" {}
variable "ami_id" {}
variable "cluster_member_sg_id" {}
variable "cluster_instance_profile_name"  {}
variable "key_name"  {}