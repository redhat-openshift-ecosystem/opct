output "aws_region" {
  value = var.aws_region
}

output "vpc_id" {
  value = aws_vpc.cluster_vpc.id
}

output "public_subnet_ids" {
  value = [
    aws_subnet.public_subnet_1.id,
    aws_subnet.public_subnet_2.id,
    aws_subnet.public_subnet_3.id,
  ]
}

output "private_subnet_ids" {
  value = [
    aws_subnet.private_subnet_1.id,
    aws_subnet.private_subnet_2.id,
    aws_subnet.private_subnet_3.id,
  ]
}

output "internet_gateway_id" {
  value = aws_internet_gateway.cluster_igw.id
}

output "public_route_table_id" {
  value = aws_route_table.public_rt.id
}

output "public_subnet_association_ids" {
  value = [
    aws_route_table_association.public_subnet_association_1.id,
    aws_route_table_association.public_subnet_association_2.id,
    aws_route_table_association.public_subnet_association_3.id,
  ]
}
