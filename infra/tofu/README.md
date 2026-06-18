# Infrastructure Bootstrap

This directory contains sanitized OpenTofu modules and examples for portable cloud substrate bootstrap.

Bootstrap output should be converted into reviewable Polykube Kubernetes manifests before being applied to clusters.

## Layout

- `modules/polykube-manifests`: provider-neutral manifest conversion for `ClusterMember` and `Federation` resources.
- `examples/aws-gcp`: reference-only AWS/GCP member wiring that uses caller-provided cluster outputs.

The examples do not install cloud providers or assume a credential manager. Cloud-specific cluster creation remains outside this repository until concrete, sanitized bootstrap modules are added.
