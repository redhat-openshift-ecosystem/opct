data "aws_route53_zone" "cluster_zone" {
  name = var.base_domain
}

resource "aws_route53_record" "api_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "api.${var.cluster_name}.${var.base_domain}."
  type    = "A"
  alias {
    name                   = var.api_lb_dns_name
    zone_id                = var.api_lb_dns_zone_id
    evaluate_target_health = false
  }
}
