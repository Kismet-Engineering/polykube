# Polykube

Polykube is an experimental open source toolkit for running backend workloads across multiple Kubernetes clusters, regions, and cloud providers without binding the application model to one vendor.

The project is starting from a private extraction phase. The first public release will focus on a small, auditable path:

- bootstrap portable cloud substrate primitives
- describe federated cluster membership declaratively
- reconcile workloads through a Kubernetes-native operator
- validate cross-cluster service routing locally before cloud rollout
- document the operational tradeoffs honestly

## Status

Private alpha scaffold. No production guarantees yet.

## Goals

- Reduce cloud and region lock-in for backend services.
- Keep the control plane Kubernetes-native and self-hostable.
- Make multicluster behavior observable, testable, and reversible.
- Provide reference patterns that organizations can adopt independently.

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
