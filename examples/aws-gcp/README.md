# AWS/GCP End-to-End Path

This example describes the reference path for connecting an AWS cluster and a GCP cluster with Polykube. It follows the four-step flow documented in `infra/tofu/README.md`.

## Step 1 — Provision clusters and networking

Create your Kubernetes clusters on AWS (e.g. EKS) and GCP (e.g. GKE). This step is outside this repository — use your team's existing cluster provisioning tooling.

Once the clusters exist:

1. Install Cilium on each cluster with ClusterMesh support enabled.
2. If the clusters can't reach each other's pod CIDRs over the public internet or a VPC peering connection, install Netmaker on a host reachable by both clusters and enroll each cluster's nodes as peers. Netmaker establishes a WireGuard overlay so cross-cluster pod traffic has a routable path.
3. Connect Cilium ClusterMesh across the two clusters so Cilium global services work.

At the end of this step you should have:
- Two clusters with reachable Kubernetes API endpoints
- Cross-cluster pod routing working (verify with a connectivity test)
- Cluster API endpoints, pod CIDRs, and service CIDRs recorded for the next step

## Step 2 — Generate Polykube manifests

The OpenTofu module at `infra/tofu/examples/aws-gcp` takes your cluster outputs and renders `ClusterMember` and `Federation` manifests:

```bash
cd infra/tofu/examples/aws-gcp
tofu init
tofu apply \
  -var="aws_api_endpoint=https://your-aws-endpoint" \
  -var="gcp_api_endpoint=https://your-gcp-endpoint"
```

Review the output manifests:

```bash
tofu output manifests
```

## Step 3 — Review and commit

Inspect the generated YAML. When satisfied, commit the manifests to your GitOps repository alongside any overlays (e.g. image tag overrides for the operator).

## Step 4 — Flux delivers to clusters

Apply the Polykube operator component from `gitops/components/operator` via Flux (or `kubectl kustomize` for a manual first apply). Flux will reconcile the CRDs and operator into each cluster. The operator then reads the `ClusterMember` and `Federation` resources and begins reconciling workload intent locally.

See `gitops/README.md` for how to wire the operator component into a Flux source.
