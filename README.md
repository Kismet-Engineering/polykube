# Polykube

Kubernetes-native infrastructure for portable backend workloads across clusters, regions, and clouds.

Polykube is an open source project for teams that want multicluster backends. It defines provider-neutral Kubernetes APIs for cluster membership, workload placement, service routing, and optional data dependency intent, then reconciles the local slice of that desired state from inside each participating cluster.

This initial version includes:

- CRDs under `polykube.dev` for federation, runtime, routing, and data intent
- workload runtime resources reconciliation through a Kubernetes operator
- local multicluster validation behavior before any cloud rollout
- cloud bootstrap output conversion into reviewable Kubernetes manifests
- runtime components provided to GitOps instead of hiding live mutations


## Goals

- Reduce cloud, region, and cluster lock-in for backend services.
- Keep desired state in Kubernetes resources and reconciled by local-cluster controllers.
- Make multicluster behavior observable, testable, and reversible before it reaches production infrastructure.
- Provide reference patterns that can be adopted independently rather than requiring a hosted product.

See `docs/decisions/0002-public-alpha-scope.md` for the first public alpha boundary and extraction rules.
Known alpha limitations are documented in `docs/known-limitations.md`.

## Initial Layout

- `operator/`: Kubernetes operator and CRD implementation.
- `infra/tofu/`: OpenTofu bootstrap modules and examples.
- `gitops/`: reusable runtime component manifests.
- `examples/local-multicluster/`: local multicluster validation harness.
- `examples/aws-gcp/`: reference AWS/GCP bootstrap path.
- `docs/`: architecture, roadmap, decisions, and contributor-facing docs.
- `scripts/`: local helper scripts.

## Release Readiness

The repository must satisfy `docs/release/public-alpha-checklist.md` before public visibility is enabled.

## Project Identity

- Repository: `github.com/Kismet-Engineering/polykube`
- Kubernetes namespace: `polykube-system`
- Kubernetes API group root: `polykube.dev`
- License: Apache-2.0
