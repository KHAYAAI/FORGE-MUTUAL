# The ledger's database: six TigerBeetle replicas on dedicated hosts, two per zone, as its documentation advises for
# production (without --development). It is outside Kubernetes on purpose: it wants whole machines, local NVMe and a
# stable address, not a scheduler. Only the ledger gateway may reach it.
resource "aws_security_group" "tigerbeetle" {
  name        = "forge-${var.environment}-tigerbeetle"
  description = "TigerBeetle: replicas talk to each other; the ledger gateway talks to them"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group_rule" "tb_peers" {
  type                     = "ingress"
  security_group_id        = aws_security_group.tigerbeetle.id
  from_port                = 3001
  to_port                  = 3001
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.tigerbeetle.id
  description              = "Replica to replica"
}

resource "aws_security_group_rule" "tb_from_ledger" {
  type                     = "ingress"
  security_group_id        = aws_security_group.tigerbeetle.id
  from_port                = 3001
  to_port                  = 3001
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.ledger_clients.id
  description              = "The ledger gateway's pods"
}

# Attach this to the ledger gateway's pods (security groups for pods); no other workload gets it.
resource "aws_security_group" "ledger_clients" {
  name        = "forge-${var.environment}-ledger-clients"
  description = "Pods allowed to reach TigerBeetle"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group_rule" "tb_egress_peers" {
  type                     = "egress"
  security_group_id        = aws_security_group.tigerbeetle.id
  from_port                = 3001
  to_port                  = 3001
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.tigerbeetle.id
  description              = "Replica to replica"
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]
  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }
}

resource "aws_iam_role" "tigerbeetle" {
  name = "forge-${var.environment}-tigerbeetle"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "ec2.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "tb_ssm" {
  role       = aws_iam_role.tigerbeetle.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore" # shell access through SSM, never SSH
}

resource "aws_iam_instance_profile" "tigerbeetle" {
  name = "forge-${var.environment}-tigerbeetle"
  role = aws_iam_role.tigerbeetle.name
}

resource "aws_instance" "tigerbeetle" {
  count                  = 6
  ami                    = data.aws_ami.al2023.id
  instance_type          = var.tigerbeetle_instance_type
  subnet_id              = aws_subnet.data[count.index % 3].id
  vpc_security_group_ids = [aws_security_group.tigerbeetle.id]
  iam_instance_profile   = aws_iam_instance_profile.tigerbeetle.name
  monitoring             = true
  ebs_optimized          = true
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required" # IMDSv2 only
    http_put_response_hop_limit = 1
  }
  root_block_device {
    encrypted   = true
    kms_key_id  = aws_kms_key.main["ledger"].arn
    volume_type = "gp3"
    volume_size = 50
  }
  tags = { Name = "forge-${var.environment}-tigerbeetle-${count.index}", Replica = tostring(count.index) }
}

# The data file lives on its own encrypted volume, snapshotted by Data Lifecycle Manager, so a replica can be rebuilt.
resource "aws_ebs_volume" "tigerbeetle_data" {
  count             = 6
  availability_zone = var.azs[count.index % 3]
  size              = 500
  type              = "io2"
  iops              = 16000
  encrypted         = true
  kms_key_id        = aws_kms_key.main["ledger"].arn
  tags              = { Name = "forge-${var.environment}-tigerbeetle-data-${count.index}", Snapshot = "daily" }
}

resource "aws_volume_attachment" "tigerbeetle_data" {
  count       = 6
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.tigerbeetle_data[count.index].id
  instance_id = aws_instance.tigerbeetle[count.index].id
}
