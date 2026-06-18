output "federation_manifest" {
  description = "Example Federation manifest."
  value       = module.polykube_manifests.federation_manifest
}

output "cluster_member_manifests" {
  description = "Example ClusterMember manifests."
  value       = module.polykube_manifests.cluster_member_manifests
}

output "manifests" {
  description = "Ordered example manifests for review or GitOps handoff."
  value       = module.polykube_manifests.manifests
}
