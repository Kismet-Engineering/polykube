# Operator Images

The Polykube operator image is built from `operator/Dockerfile` and published by `.github/workflows/ci.yml`.

## Image Repository

Published images use GitHub Container Registry:

```text
ghcr.io/kismet-engineering/polykube-operator
```

The reusable GitOps component defaults to a published alpha image for live cloud installs:

```text
ghcr.io/kismet-engineering/polykube-operator:0.1.0-alpha.1
```

Use this default only when you intentionally want that alpha release. For a real live-cloud environment, prefer an overlay that pins the exact published tag being promoted.

Local development is separate and defaults to a local image name:

```text
polykube-operator:dev
```

Do not commit `polykube-operator:dev` into a live cloud GitOps path. Use it only with local render/deploy commands after building and loading the image into the target local cluster runtime.

## Version Tags

The CI workflow uses this tag scheme:

- Pull requests: build only, no publish.
- `main`: publish `edge` and `sha-<shortsha>`.
- Git tags matching `v*`: publish semantic tags from the Git tag.

Examples:

- `v0.1.0-alpha.1` publishes `0.1.0-alpha.1`.
- A merge to `main` publishes `edge` and a commit-pinned `sha-<shortsha>` tag.

Use immutable `sha-<shortsha>` tags for reproducible GitOps promotion. Use `edge` only for disposable development environments.

## Runtime Policy

- Live cloud default: `kubectl kustomize gitops/components/operator` renders the release image declared in `gitops/components/operator/deployment.yaml`.
- Live cloud promotion: use a kustomize image overlay to pin a published `sha-<shortsha>` or semantic release tag.
- Local development: use `polykube-operator:dev` through `mise run operator:render`, `mise run operator:deploy`, or `mise run local:operator:deploy` after building/loading the image locally.

## Local Commands

Build and test the operator:

```bash
mise run operator:test
mise run operator:build
```

Build a local image:

```bash
mise run operator:image:build -- --image polykube-operator:dev
```

Render manifests for a specific image:

```bash
mise run operator:render -- --image ghcr.io/kismet-engineering/polykube-operator:sha-<shortsha>
```

Render manifests for local development:

```bash
mise run operator:render -- --image polykube-operator:dev
```

Deploy a local development image to the current Kubernetes context:

```bash
mise run operator:deploy -- --image polykube-operator:dev
```

Remove the operator from the current Kubernetes context:

```bash
mise run operator:undeploy
```
