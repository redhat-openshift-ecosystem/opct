output "api_lb_name" {
  value = aws_lb.api_nlb.name
}

output "api_lb_dns_name" {
  value = aws_lb.api_nlb.dns_name
}

output "api_lb_zone_id" {
  value = aws_lb.api_nlb.zone_id
}

