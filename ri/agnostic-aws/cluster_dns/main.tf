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

# TODO
# resource "aws_route53_record" "api_record_ptr" {
#  zone_id =  data.aws_route53_zone.cluster_zone.id
#  name    = "${join(".", reverse(split(".", aws_route53_record.api_record_a.name)))}.in-addr.arpa."
#  type    = "PTR"
#  ttl     = 300

#  records = [var.ptr_record_value]
# }

resource "aws_route53_record" "api_int_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "api-int.${var.cluster_name}.${var.base_domain}."
  type    = "A"
  alias {
    name                   = var.api_lb_dns_name
    zone_id                = var.api_lb_dns_zone_id
    evaluate_target_health = false
  }
}

# TODO: Review DNS Targets

resource "aws_route53_record" "apps_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "*.apps.${var.cluster_name}.${var.base_domain}."
  type    = "A"
  alias {
    name                   = var.api_lb_dns_name
    zone_id                = var.api_lb_dns_zone_id
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "bootstrap_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "bootstrap.${var.cluster_name}.${var.base_domain}."
  type    = "A"
  alias {
    name                   = var.api_lb_dns_name
    zone_id                = var.api_lb_dns_zone_id
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "bootstrap_record_a" {
  zone_id = data.aws_route53_zone.cluster_zone.id
  name    = "bootstrap.${var.cluster_name}.${var.base_domain}."
  type    = "A"
  alias {
    name                   = var.api_lb_dns_name
    zone_id                = var.api_lb_dns_zone_id
    evaluate_target_health = false
  }
}

# TODO: <master><n>.<cluster_name>.<base_domain>.
# TODO: <worker><n>.<cluster_name>.<base_domain>.
