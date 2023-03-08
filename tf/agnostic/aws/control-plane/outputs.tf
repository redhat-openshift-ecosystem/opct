output "control_plane_instance_ids" {
  value = { for idx, instance in module.control_plane_instances : "control_plane_${idx}" => instance.instance_id }
}