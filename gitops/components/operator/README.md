# Operator Component

This component installs the Polykube operator runtime objects for GitOps reconciliation.

## Required Inputs

- Operator image: replace the placeholder `polykube-operator:dev` with an image built by your release pipeline.
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

Apply through your GitOps controller after substituting the operator image in an overlay.
