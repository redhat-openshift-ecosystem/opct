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

variable "ami_mapping" {
  type = map(map(string))
  default = {
    "us-west-2" = {
      "x86_64" = "ami-08970fb2e5767e3b8"
      "arm64"  = "ami-0bb199dd39edd7d71"
    }
  }
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

module "control_plane" {
  source = "./control-plane"
  instance_type = var.instance_type
  subnet_id = module.networking.public_subnet_ids[0]
  cluster_member_sg_id = module.security.cluster_member_sg_id
  cluster_instance_profile_name = module.security.cluster_instance_profile_name
  ami_id = var.ami_mapping[var.aws_region][var.architecture]
  key_name = module.security.key_name
  depends_on = [ module.security ]
}

module "workers" {
  source = "./workers"
  instance_type = var.instance_type
  subnet_id = module.networking.public_subnet_ids[0]
  cluster_member_sg_id = module.security.cluster_member_sg_id
  cluster_instance_profile_name = module.security.cluster_instance_profile_name
  ami_id = var.ami_mapping[var.aws_region][var.architecture]
  key_name = module.security.key_name
  depends_on = [ module.security ]
}



