# Infrastructure — Manifest Generation

This directory contains OpenTofu modules that convert provisioned cluster details into Polykube `ClusterMember` and `Federation` manifests. It is one step in a larger flow:

```
Step 1 — Provision clusters and networking (outside this repo)
         Create Kubernetes clusters on AWS, GCP, or elsewhere.
         Install Cilium with ClusterMesh enabled on each cluster.
         Connect inter-node networking (e.g. Netmaker/WireGuard) if clusters
         don't share a network and can't reach each other's pod CIDRs directly.
         → produces: cluster API endpoints, pod CIDRs, service CIDRs

Step 2 — Generate Polykube manifests (this directory)
         Feed cluster outputs into the polykube-manifests module.
         → produces: ClusterMember + Federation YAML for review

Step 3 — Review and commit
         Inspect the generated YAML. Commit to your GitOps repository.

Step 4 — Flux delivers to clusters (see gitops/)
         Flux reconciles the Polykube operator and CRDs into each cluster.
         The operator starts reconciling workload intent.
```

The modules here handle Step 2 only. They do not create clusters, networks, IAM roles, DNS records, or certificates.

## Prerequisites for Step 2

Before running these modules, you need:

- Kubernetes clusters with reachable API endpoints
- Cilium installed and ClusterMesh connected across clusters
- Netmaker (or equivalent) configured if clusters don't share a network
- Cluster API endpoints, pod CIDRs, and service CIDRs available as inputs

## Layout

- `modules/polykube-manifests`: provider-neutral module that accepts cluster outputs and renders `ClusterMember` and `Federation` YAML.
- `examples/aws-gcp`: reference wiring for AWS and GCP clusters, showing how to pass cluster outputs into the module.

## Usage

See `examples/aws-gcp/` for a concrete invocation. The general pattern:

```hcl
module "polykube_manifests" {
  source = "path/to/modules/polykube-manifests"

  federation_name      = "my-federation"
  routing_mode         = "ActiveActive"
  networking_substrate = "cilium-clustermesh"

  members = {
    cluster-a = {
      provider     = "aws"
      region       = "us-east-1"
      api_endpoint = "https://..."
      # ...
    }
  }
}
```

Outputs include `federation_manifest`, `cluster_member_manifests`, and `manifests` (ordered, ready for GitOps handoff).
