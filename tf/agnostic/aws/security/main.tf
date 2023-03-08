# terraform apply -auto-approve

resource "aws_security_group" "cluster_member_sg" {
  name_prefix = "ssh_access_"
  vpc_id      = var.vpc_id
}

resource "aws_security_group_rule" "allow_icmp" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = -1
  to_port           = -1
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow_ssh" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow_http" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow_https" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow_metrics" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 1936
  to_port           = 1936
  protocol          = "tcp"
  self              = true
}

resource "aws_security_group_rule" "allow_host_services" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 9000
  to_port           = 9999
  protocol          = "tcp"
  self              = true
}

resource "aws_security_group_rule" "allow_k8s" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 10250
  to_port           = 10259
  protocol          = "tcp"
  self              = true
}

resource "aws_security_group_rule" "allow_sdn" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 10256
  to_port           = 10256
  protocol          = "tcp"
  self              = true
}

resource "aws_security_group_rule" "allow_k8s_node" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 30000
  to_port           = 32767
  protocol          = "tcp"
  self              = true
}

resource "aws_security_group_rule" "allow_udp_k8s_node" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 30000
  to_port           = 32767
  protocol          = "udp"
  self              = true
}


resource "aws_security_group_rule" "allow_vxlan" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 4789
  to_port           = 4789
  protocol          = "udp"
  self              = true
}

resource "aws_security_group_rule" "allow_vxlan_geneve" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 6081
  to_port           = 6081
  protocol          = "udp"
  self              = true
}

resource "aws_security_group_rule" "allow_udp_host_services" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 9000
  to_port           = 9999
  protocol          = "udp"
  self              = true
}

resource "aws_security_group_rule" "allow_udp_ipsec" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 500
  to_port           = 500
  protocol          = "udp"
  self              = true
}

resource "aws_security_group_rule" "allow_udp_ipsec-nat" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = 4500
  to_port           = 4500
  protocol          = "udp"
  self              = true
}

resource "aws_security_group_rule" "allow_esp" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "ingress"
  from_port         = -1
  to_port           = -1
  protocol          = "50"
  self              = true
}

resource "aws_security_group_rule" "allow_outbound_tcp" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow_outbound_udp" {
  security_group_id = aws_security_group.cluster_member_sg.id
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
}

#TODO: Set unique names
resource "aws_iam_instance_profile" "cluster_instance_profile" {
  name = "cluster-instance-profile"
  role = aws_iam_role.cluster_role.id
}

resource "aws_iam_role" "cluster_role" {
  name = "cluster-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy_attachment" "cluster_policy_attachment" {
  name       = "example-policy-attachment"
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
  roles      = [aws_iam_role.cluster_role.name]
}


resource "tls_private_key" "cluster_key" {
  algorithm   = "RSA"
  rsa_bits    = 4096
}

resource "aws_key_pair" "cluster_key_pair" {
  key_name   = "cluster_key_pair"
  public_key = tls_private_key.cluster_key.public_key_openssh
}



