# AWS/GCP Example

This example documents the reference AWS/GCP bootstrap path as optional example wiring, not a required product assumption.

- OpenTofu manifest conversion root: `infra/tofu/examples/aws-gcp`
- GitOps operator component: `gitops/components/operator`

The OpenTofu example consumes caller-provided cluster outputs and renders Polykube `ClusterMember` and `Federation` manifests for review before GitOps handoff.
