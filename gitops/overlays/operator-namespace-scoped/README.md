# Namespace-Scoped Operator Overlay

This overlay runs one Polykube operator in `polykube-system` and restricts all namespaced watches and application permissions to `polykube-workloads`. `ClusterMember` and `Federation` remain cluster-scoped because their CRDs and controllers are cluster-scoped.

Apply the CRDs first, then render or apply the overlay:

```bash
kubectl apply -f operator/config/crd/bases
kubectl kustomize gitops/overlays/operator-namespace-scoped
kubectl kustomize gitops/overlays/operator-namespace-scoped | kubectl apply -f -
```

To use another workload namespace, copy this overlay and change the namespace consistently in:

- `workload-namespace.yaml`
- `workload-role.yaml`
- `workload-role-binding.yaml`
- the `--watch-namespace` argument in `deployment.yaml`

Keep the operator ServiceAccount and leader-election resources in `polykube-system`. All namespaced Polykube resources and their referenced Secrets and ConfigMaps must be in the selected workload namespace. Cross-namespace references do not work in this profile by design.

See `docs/security.md` for permissions, trust assumptions, and verification commands.
