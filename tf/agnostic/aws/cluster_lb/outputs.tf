output "api_lb_name" {
  value = aws_lb.api_nlb.name
}

output "api_lb_dns_name" {
  value = aws_lb.api_nlb.dns_name
}
