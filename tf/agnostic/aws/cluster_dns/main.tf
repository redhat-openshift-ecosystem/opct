data "aws_route53_zone" "cluster_zone" {
  name = var.base_domain
}

resource "aws_route53_record" "api_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "api.${var.base_domain}."
  type    = "A"
  alias {
    name                   = "api.${var.base_domain}."
    zone_id                = data.aws_route53_zone.cluster_zone.id
    evaluate_target_health = false
  }
}
