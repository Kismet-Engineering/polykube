# GitOps Components

This directory contains runtime manifests for deploying the Polykube operator via Flux or any kustomize-compatible GitOps tool. It is Step 4 of the deployment flow described in `infra/tofu/README.md`.

## Components

- `components/operator`: installs the Polykube operator and its runtime objects into the `polykube-system` namespace.
- `overlays/operator-namespace-scoped`: restricts namespaced watches and permissions to `polykube-workloads` while retaining required access to cluster-scoped Polykube infrastructure resources.

CRD manifests live under `operator/config/crd/bases` and should be applied before the operator component. Generated CRD packaging is a follow-up once the API generation workflow is introduced.

## Using with Flux

Point a Flux `Kustomization` at `gitops/components/operator` in your GitOps repository. The component defaults to the published alpha image declared in `deployment.yaml`, which is intended for live cloud GitOps installs and manual live-cloud applies.

For reproducible promotion, use an overlay that pins the exact published tag you want to run. See `docs/release/operator-images.md` for tag conventions.

The base component has cluster-wide namespaced access. Prefer the namespace-scoped overlay when all managed workloads can share one namespace. Its customization requirements and remaining trust boundaries are documented in `docs/security.md`.

A minimal overlay `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - github.com/Kismet-Engineering/polykube/gitops/components/operator
images:
  - name: ghcr.io/kismet-engineering/polykube-operator
    newTag: sha-<shortsha>  # replace with your promoted image tag
```

Local development is separate from the live-cloud GitOps path. Build and deploy a local image with the local scripts instead of committing `polykube-operator:dev` to a GitOps repository:

```bash
mise run operator:image:build -- --image polykube-operator:dev
mise run operator:render -- --image polykube-operator:dev
```

## Manual apply

To apply without Flux:

```bash
kubectl kustomize gitops/components/operator | kubectl apply -f -
```

To install the restricted profile with its default `polykube-workloads` namespace:

```bash
kubectl kustomize gitops/overlays/operator-namespace-scoped | kubectl apply -f -
```

Operator image publishing and tag conventions are documented in `docs/release/operator-images.md`.
