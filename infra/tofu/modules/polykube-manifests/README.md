# Polykube Manifest Module

This provider-neutral module converts cluster bootstrap outputs into Polykube Kubernetes manifests.

## Inputs

- `federation_name`: name for the generated `Federation` resource.
- `routing_mode`: `ActivePassive` or `ActiveActive`. Rendered as the metadata-only `Federation.spec.routingMode`; routing is reconciled from `ServiceEndpoint.spec.routingMode`.
- `networking_substrate`: descriptive substrate name, for example `cilium-clustermesh`.
- `networking_details`: optional string map for substrate-specific metadata.
- `members`: map of cluster member descriptions keyed by the desired `ClusterMember` name. Member `labels` are rendered into both `metadata.labels`, which selectors match, and the metadata-only `spec.labels`.

The `networking` block and member `zone`, `environment`, `api_endpoint`, `pod_cidr`, and `service_cidr` values are descriptive metadata. The operator does not act on them. See [`docs/api-field-support.md`](../../../../docs/api-field-support.md).

## Outputs

- `federation_manifest`: YAML for one `Federation` resource.
- `cluster_member_manifests`: map of YAML `ClusterMember` resources keyed by member name.
- `manifests`: ordered list containing the federation manifest followed by cluster member manifests.

Review the rendered YAML before applying it through GitOps or `kubectl`.
