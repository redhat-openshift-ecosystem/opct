
resource "aws_vpc" "cluster_vpc" {
  cidr_block = var.vpc_cidr_block
  tags = {
    Name = "cluster_vpc"
  }
}

# Create three public subnets
resource "aws_subnet" "public_subnet_1" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.public_subnet_cidr_blocks[0]
  availability_zone = var.public_az_1
  map_public_ip_on_launch = true
  tags = {
    Name = "public_subnet_1"
  }
}

resource "aws_subnet" "public_subnet_2" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.public_subnet_cidr_blocks[1]
  availability_zone = var.public_az_2
  map_public_ip_on_launch = true
  tags = {
    Name = "public_subnet_2"
  }
}

resource "aws_subnet" "public_subnet_3" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.public_subnet_cidr_blocks[2]
  availability_zone = var.public_az_3
  map_public_ip_on_launch = true
  tags = {
    Name = "public_subnet_3"
  }
}

# Create three private subnets
resource "aws_subnet" "private_subnet_1" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.private_subnet_cidr_blocks[0]
  availability_zone = var.private_az_1
  tags = {
    Name = "private_subnet_1"
  }
}

resource "aws_subnet" "private_subnet_2" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.private_subnet_cidr_blocks[1]
  availability_zone = var.private_az_2
  tags = {
    Name = "private_subnet_2"
  }
}

resource "aws_subnet" "private_subnet_3" {
  vpc_id            = aws_vpc.cluster_vpc.id
  cidr_block        = var.private_subnet_cidr_blocks[2]
  availability_zone = var.private_az_3
  tags = {
    Name = "private_subnet_3"
  }
}

# Create an internet gateway and attach it to the VPC
resource "aws_internet_gateway" "cluster_igw" {
  vpc_id = aws_vpc.cluster_vpc.id
  tags = {
    Name = "cluster_igw"
  }
}

# Create a route table and associate the public subnets with it
resource "aws_route_table" "public_rt" {
  vpc_id = aws_vpc.cluster_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.cluster_igw.id
  }

  tags = {
    Name = "public_rt"
  }
}

resource "aws_route_table_association" "public_subnet_association_1" {
  subnet_id = aws_subnet.public_subnet_1.id
  route_table_id = aws_route_table.public_rt.id

  depends_on = [
    aws_route_table.public_rt,
  ]
}

resource "aws_route_table_association" "public_subnet_association_2" {
  subnet_id = aws_subnet.public_subnet_2.id
  route_table_id = aws_route_table.public_rt.id

  depends_on = [
    aws_route_table.public_rt,
  ]
}

resource "aws_route_table_association" "public_subnet_association_3" {
  subnet_id = aws_subnet.public_subnet_3.id
  route_table_id = aws_route_table.public_rt.id

  depends_on = [
    aws_route_table.public_rt,
  ]
}

