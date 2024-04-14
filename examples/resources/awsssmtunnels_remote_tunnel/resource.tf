##############################################
######## Prerequisites #######################
##############################################

// Make sure you have one or more bastion instances running in the correct VPCs and with the correct
// Security groups to allow connectivity to EKS/RDS and whatever else you are trying to connect to.
// In the examples below, we have a single bastion instance with the ID i-123456789. The instance
// has 2 security groups applied. One allows port 443 outbound to EKS (and EKS's security group allows
// 443 inbound from the bastion's security group). The bastion has another security group that allows port
// 5432 outbound to RDS (and RDS's security group allows 5432 inbound from the bastion's security group).

##############################################
######## EKS Example #########################
##############################################

provider "kubernetes" {
  host                   = "https://${awsssmtunnels_remote_tunnel.eks.local_host}:${awsssmtunnels_remote_tunnel.eks.local_port}"
  tls_server_name        = replace(aws_eks_cluster.example.endpoint, "https://", "")
  cluster_ca_certificate = base64decode(aws_eks_cluster.example.certificate_authority.0.data)
  token                  = data.aws_eks_cluster_auth.example.token
}

data "aws_eks_cluster_auth" "example" {
  name = aws_eks_cluster.example.name
}

resource "null_resource" "trigger_refresh" {
  triggers = {
    always_change = "${timestamp()}"
  }
}

resource "awsssmtunnels_remote_tunnel" "eks" {
  target      = "i-123456789"
  remote_host = replace(aws_eks_cluster.example.endpoint, "https://", "")
  remote_port = 443
  local_port  = 16534
  region      = "us-east-1"

  lifecycle {
    replace_triggered_by = [null_resource.trigger_refresh]
  }
}

// NOTE: We use the *_keepalive data resource to prevent the provider from being shut down prematurely
// We need the tunnel to stay up until all the resources for the providers using the tunnel are done
// reading or writing from it.
data "awsssmtunnels_keepalive" "eks" {
  depends_on = [
    kubernetes_secret.one,
    kubernetes_secret.two,
    kubernetes_config_map.one,
    kubernetes_config_map.two,
    helm_release.example_operator,
    awsssmtunnels_remote_tunnel.eks,
  ]
}


##############################################
######## RDS Example #########################
##############################################

resource "awsssmtunnels_remote_tunnel" "rds" {
  target      = "i-123456789"
  remote_host = aws_rds_cluster.example.endpoint
  remote_port = 5432 // This is a PostgreSQL RDS cluster example
  local_port  = 17638
  region      = "us-east-1"

  lifecycle {
    replace_triggered_by = [null_resource.trigger_refresh]
  }
}

data "awsssmtunnels_keepalive" "rds" {
  depends_on = [
    postgresql_tables.my_tables,
    awsssmtunnels_remote_tunnel.rds,
  ]
}

provider "postgresql" {
  host            = awsssmtunnels_remote_tunnel.rds.local_host
  port            = awsssmtunnels_remote_tunnel.rds.local_port
  database        = "mydb"
  username        = var.pg_user
  password        = var.pg_password
  sslmode         = "require"
  connect_timeout = 15
}

data "postgresql_tables" "my_tables" {
  database   = "mydb"
  depends_on = [awsssmtunnels_remote_tunnel.rds] // NOTE: The tunnel must be up before we can query the database
}
