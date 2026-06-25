# Operator Images

The Polykube operator image is built from `operator/Dockerfile` and published by `.github/workflows/ci.yml`.

## Image Repository

Published images use GitHub Container Registry:

```text
ghcr.io/kismet-engineering/polykube-operator
```

Local development defaults to:

```text
polykube-operator:dev
```

## Version Tags

The CI workflow uses this tag scheme:

- Pull requests: build only, no publish.
- `main`: publish `edge` and `sha-<shortsha>`.
- Git tags matching `v*`: publish semantic tags from the Git tag.

Examples:

- `v0.1.0` publishes `0.1.0` and `0.1`.
- A merge to `main` publishes `edge` and a commit-pinned `sha-<shortsha>` tag.

Use immutable `sha-<shortsha>` tags for reproducible GitOps promotion. Use `edge` only for disposable development environments.

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

Deploy to the current Kubernetes context:

```bash
mise run operator:deploy -- --image polykube-operator:dev
```

Remove the operator from the current Kubernetes context:

```bash
mise run operator:undeploy
```
