# GitOps Components

This directory contains reusable runtime manifests intended for GitOps reconciliation.

## Components

- `components/operator`: installs the Polykube operator runtime objects in `polykube-system`.

CRD manifests currently live under `operator/config/crd/bases` and should be reconciled before the operator component. Generated CRD packaging is a follow-up once the API generation workflow is introduced.

Operator image publishing and tag conventions are documented in `docs/release/operator-images.md`.
