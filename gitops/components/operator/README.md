# Operator Component

This component installs the Polykube operator runtime objects for GitOps reconciliation.

## Required Inputs

- Operator image: the component defaults to the published alpha image in `deployment.yaml` for live cloud GitOps installs. Pin a different published tag in an overlay when promoting a specific build.
- Namespace: defaults to `polykube-system`; overlays may change it if every referenced object is updated consistently.
- CRDs: apply `operator/config/crd/bases` before this component so the operator can watch Polykube resources.

## Ownership Boundaries

- This component owns the operator `Namespace`, `ServiceAccount`, `ClusterRole`, `ClusterRoleBinding`, and `Deployment`.
- It does not own cloud infrastructure, CNI installation, cluster bootstrap, workload CRs, or external rollout controllers.
- It grants the operator local-cluster permissions only; credentials for remote clusters are intentionally out of scope.

## Usage

```bash
kubectl kustomize gitops/components/operator
```

Apply through your GitOps controller directly for the default alpha image, or add an image overlay that pins the exact published tag you want to promote. Local demos should not consume the default directly; they use `mise run operator:render -- --image polykube-operator:dev` or `mise run local:operator:deploy -- --image polykube-operator:dev` after building and loading the local image.

Published image tags and local build commands are documented in `docs/release/operator-images.md`.
