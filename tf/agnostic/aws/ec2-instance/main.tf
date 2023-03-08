resource "aws_instance" "bootstrap_instance" {
  ami           = var.ami_id
  instance_type = var.instance_type
  subnet_id     = var.subnet_id
  security_groups = [var.cluster_member_sg_id]
  iam_instance_profile = var.cluster_instance_profile_name
  key_name = var.key_name
  user_data              = <<EOF
#!/bin/bash
amazon-linux-extras install -y epel
yum install -y https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_amd64/amazon-ssm-agent.rpm
systemctl enable amazon-ssm-agent
systemctl start amazon-ssm-agent
EOF

  tags = {
    Name = "openshift ec2 instance"
  }
}
