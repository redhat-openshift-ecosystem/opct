output "workers_instance_ids" {
  value = { for idx, instance in module.workers_instances : "workers_${idx}" => instance.instance_id }
}