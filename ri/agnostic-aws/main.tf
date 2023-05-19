# tf apply -auto-approve
provider "aws" {
  region = var.aws_region
}

module "networking" {
  source = "./networking"
  aws_region = var.aws_region
}

module "security" {
  source = "./security"
  aws_region = var.aws_region
  vpc_id = module.networking.vpc_id

  depends_on = [ module.networking ]
}


module "bootstrap" {
  source = "./bootstrap"
  instance_type = var.instance_type
  subnet_id = module.networking.public_subnet_ids[0]
  cluster_member_sg_id = module.security.cluster_member_sg_id
  cluster_instance_profile_name = module.security.cluster_instance_profile_name
  ami_id = var.ami_mapping[var.aws_region][var.architecture]
  key_name = module.security.key_name
  depends_on = [ module.security ]
}

#module "control_plane" {
#  source = "./control_plane"
#  instance_type = var.instance_type
#  subnet_ids = module.networking.public_subnet_ids
#  cluster_member_sg_id = module.security.cluster_member_sg_id
#  cluster_instance_profile_name = module.security.cluster_instance_profile_name
#  ami_id = var.ami_mapping[var.aws_region][var.architecture]
#  key_name = module.security.key_name
#  depends_on = [ module.security ]
#}
#
#module "workers" {
#  source = "./workers"
#  instance_type = var.instance_type
#  subnet_id = module.networking.public_subnet_ids[0]
#  cluster_member_sg_id = module.security.cluster_member_sg_id
#  cluster_instance_profile_name = module.security.cluster_instance_profile_name
#  ami_id = var.ami_mapping[var.aws_region][var.architecture]
#  key_name = module.security.key_name
#  depends_on = [ module.security ]
#}
#
#module "cluster_lb" {
#  source = "./cluster_lb"
#  vpc_id = module.networking.vpc_id
#  # TODO: Consider using private subnets
#  subnet_ids = module.networking.public_subnet_ids
#  control_plane_instances_ids = module.control_plane.control_plane_instance_ids
#}
#
#module "cluster_dns" {
#  source = "./cluster_dns"
#  cluster_name        = var.cluster_name
#  base_domain         = var.base_domain
#  api_lb_dns_name     = module.cluster_lb.api_lb_dns_name
#  api_lb_dns_zone_id  = module.cluster_lb.api_lb_zone_id
#}
#
#
#
