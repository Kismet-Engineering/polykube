# GitOps Components

This directory contains runtime manifests for deploying the Polykube operator via Flux or any kustomize-compatible GitOps tool. It is Step 4 of the deployment flow described in `infra/tofu/README.md`.

## Components

- `components/operator`: installs the Polykube operator and its runtime objects into the `polykube-system` namespace.

CRD manifests live under `operator/config/crd/bases` and should be applied before the operator component. Generated CRD packaging is a follow-up once the API generation workflow is introduced.

## Using with Flux

Point a Flux `Kustomization` at `gitops/components/operator` in your GitOps repository. You will need an image overlay to replace the placeholder tag (`polykube-operator:dev`) with a published release image. See `docs/release/operator-images.md` for tag conventions.

A minimal overlay `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - github.com/Kismet-Engineering/polykube/gitops/components/operator
images:
  - name: polykube-operator
    newTag: v0.1.0  # replace with your target release tag
```

## Manual apply

To apply without Flux:

```bash
kubectl kustomize gitops/components/operator | kubectl apply -f -
```

Operator image publishing and tag conventions are documented in `docs/release/operator-images.md`.
