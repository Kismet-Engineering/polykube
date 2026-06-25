module "polykube_manifests" {
  source = "../../modules/polykube-manifests"

  federation_name      = var.federation_name
  routing_mode         = "ActiveActive"
  networking_substrate = "cilium-clustermesh"
  networking_details = {
    example = "aws-gcp"
  }

  members = {
    aws = {
      provider     = "aws"
      region       = var.aws_region
      zone         = ""
      environment  = var.environment
      cluster_name = "aws"
      api_endpoint = var.aws_api_endpoint
      pod_cidr     = ""
      service_cidr = ""
      labels = {
        cloud       = "aws"
        environment = var.environment
      }
    }
    gcp = {
      provider     = "gcp"
      region       = var.gcp_region
      zone         = ""
      environment  = var.environment
      cluster_name = "gcp"
      api_endpoint = var.gcp_api_endpoint
      pod_cidr     = ""
      service_cidr = ""
      labels = {
        cloud       = "gcp"
        environment = var.environment
      }
    }
  }
}
