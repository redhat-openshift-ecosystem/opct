output "vpc_id" {
  value = module.networking.vpc_id
}

output "cluster_member_sg_id" {
  value = module.security.cluster_member_sg_id
}

output "bootstrap_instance_id" {
  value = module.bootstrap.bootstrap_instance_id
}

output "bootstrap_instance_url" {
  value = "https://${var.aws_region}.console.aws.amazon.com/ec2/home?region=${var.aws_region}#InstanceDetails:instanceId=${module.bootstrap.bootstrap_instance_id}"
}

output "bootstrap_instance_ssm_url" {
  value = "https://${var.aws_region}.console.aws.amazon.com/systems-manager/session-manager/${module.bootstrap.bootstrap_instance_id}?region=${var.aws_region}#"
}

output "control_plane_instance_ids" {
  value = module.control_plane.control_plane_instance_ids
}

output "control_plane_instance_ssm_urls" {
  value = { for idx, instance_id in module.control_plane.control_plane_instance_ids :
    "control_plane_ssm_${idx}"
      => "https://${var.aws_region}.console.aws.amazon.com/systems-manager/session-manager/${instance_id}?region=${var.aws_region}#" }
}

output "worker_instance_ids" {
  value = module.control_plane.control_plane_instance_ids
}

output "worker_urls" {
  value = { for idx, instance_id in module.control_plane.control_plane_instance_ids :
  "worker_ssm_${idx}"
  => "https://${var.aws_region}.console.aws.amazon.com/systems-manager/session-manager/${instance_id}?region=${var.aws_region}#" }
}
