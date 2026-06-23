# Polykube

Kubernetes-native infrastructure for portable backend workloads across clusters, regions, and clouds.

Polykube is a fresh-start open source project for teams that want multicluster backend patterns they can understand, run, and audit themselves. It defines provider-neutral Kubernetes APIs for cluster membership, workload placement, service routing, and optional data dependency intent, then reconciles the local slice of that desired state from inside each participating cluster.

Polykube is not a hosted control plane, a SaaS product, or a new cloud abstraction layer. The public alpha is intentionally small: prove the reusable Kubernetes pattern, keep cloud assumptions visible, and make every bootstrap, routing, and reconciliation step reviewable.

The first public release focuses on a small, auditable path:

- define CRDs under `polykube.dev` for federation, runtime, routing, and data intent
- reconcile workload runtime resources through a Kubernetes operator
- validate local multicluster behavior before any cloud rollout
- convert cloud bootstrap outputs into reviewable Kubernetes manifests
- hand runtime components to GitOps instead of hiding live mutations
- document operational tradeoffs and known limitations directly

## Status

Private alpha scaffold. No production guarantees yet.

## Goals

- Reduce cloud, region, and cluster lock-in for backend services.
- Keep desired state in Kubernetes resources and reconciled by local-cluster controllers.
- Make multicluster behavior observable, testable, and reversible before it reaches production infrastructure.
- Provide reference patterns that can be adopted independently rather than requiring a hosted product.

## Non-Goals

- Polykube is not a managed SaaS.
- Polykube is not a general-purpose cloud control plane.
- Polykube is not a replacement for Kubernetes, CNI, GitOps, or cloud infrastructure tooling.
- Polykube will not hide hard operational tradeoffs behind vague automation.

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
