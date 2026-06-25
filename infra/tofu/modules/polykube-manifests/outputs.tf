output "federation_manifest" {
  description = "YAML manifest for the Polykube Federation resource."
  value       = local.federation_manifest
}

output "cluster_member_manifests" {
  description = "YAML manifests for Polykube ClusterMember resources keyed by member name."
  value       = local.cluster_member_manifests
}

output "manifests" {
  description = "Ordered YAML manifests for review or GitOps handoff."
  value = concat(
    [local.federation_manifest],
    [for name in sort(keys(local.cluster_member_manifests)) : local.cluster_member_manifests[name]],
  )
}
