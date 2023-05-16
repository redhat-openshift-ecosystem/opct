output "cluster_member_sg_id" {
  value = aws_security_group.cluster_member_sg.id
}

output "cluster_instance_profile_name" {
  value = aws_iam_instance_profile.cluster_instance_profile.name
}

output "public_key" {
  value = aws_key_pair.cluster_key_pair.public_key
}

output "private_key" {
  value = tls_private_key.cluster_key.private_key_pem
}

output "key_name" {
  value = aws_key_pair.cluster_key_pair.key_name
}