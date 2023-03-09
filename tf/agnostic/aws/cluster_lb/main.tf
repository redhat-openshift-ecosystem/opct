resource "aws_lb" "api_nlb" {
  name               = "api-network-lb"
  internal           = false
  load_balancer_type = "network"

  dynamic "subnet_mapping" {
    for_each = var.subnet_ids
    content {
      subnet_id = subnet_mapping.value
    }
  }
}

resource "aws_lb_listener" "control_plane" {
  count = length(var.control_plane_instances_ids)

  load_balancer_arn = aws_lb.api_nlb.arn
  port              = 6443
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.control_plane.arn
  }
}


resource "aws_lb_target_group" "control_plane" {
  name_prefix      = "ocpcp-"
  port             = 6443
  protocol         = "TCP"
  target_type      = "instance"
  vpc_id           = var.vpc_id
  deregistration_delay = 30

  health_check {
    healthy_threshold   = 4
    unhealthy_threshold = 2
    interval            = 60
    timeout             = 15
    protocol            = "TCP"
  }
}

# resource "aws_lb_target_group_attachment" "control_plane" {
#  count = length(var.control_plane_instances_ids)
#
#  target_group_arn = aws_lb_target_group.control_plane.arn
#  target_id        = var.control_plane_instances_ids[count.index]
#  port             = 6443
#}
