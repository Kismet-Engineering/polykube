locals {
  common_labels = {
    "app.kubernetes.io/part-of" = "polykube"
    "polykube.dev/federation"    = var.federation_name
  }

  cluster_member_objects = {
    for name, member in var.members : name => {
      apiVersion = "infrastructure.polykube.dev/v1alpha1"
      kind       = "ClusterMember"
      metadata = {
        name   = name
        labels = merge(local.common_labels, member.labels)
      }
      spec = {
        provider    = member.provider
        region      = member.region
        zone        = member.zone
        environment = member.environment
        clusterName = member.cluster_name
        apiEndpoint = member.api_endpoint
        podCIDR     = member.pod_cidr
        serviceCIDR = member.service_cidr
        labels      = member.labels
      }
    }
  }

  federation_object = {
    apiVersion = "infrastructure.polykube.dev/v1alpha1"
    kind       = "Federation"
    metadata = {
      name   = var.federation_name
      labels = local.common_labels
    }
    spec = {
      members = [
        for name in sort(keys(var.members)) : {
          name = name
        }
      ]
      routingMode = var.routing_mode
      networking = {
        substrate = var.networking_substrate
        details   = var.networking_details
      }
    }
  }

  federation_manifest      = yamlencode(local.federation_object)
  cluster_member_manifests = { for name, object in local.cluster_member_objects : name => yamlencode(object) }
}
