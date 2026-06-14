# Local Multicluster Example

This example is the first repeatable Polykube validation path. The initial pass supports local k0s cluster lifecycle; CNI, ClusterMesh, and workload routing probes will be added in follow-up passes.

## Prerequisites

- Docker-compatible runtime
- `kubectl`
- `mise` for task execution
- `colima` on macOS when using Colima as the Docker runtime

## Create Clusters

```bash
mise run local:cluster:create -- --clusters alpha,beta --workers 0
mise run local:cluster:status
```

Kubeconfigs are written under:

```text
examples/local-multicluster/state/kubeconfigs/
```

Use all local demo kubeconfigs:

```bash
export KUBECONFIG=$(ls -1 examples/local-multicluster/state/kubeconfigs/*.yaml | paste -sd: -)
```

## Recreate Or Delete

```bash
mise run local:cluster:recreate -- --clusters alpha,beta --workers 0
mise run local:cluster:delete -- --clusters alpha,beta
```

Delete all local Polykube k0s clusters:

```bash
mise run local:cluster:delete -- --all
```

## Target Proof

- two local Kubernetes clusters
- cross-cluster networking substrate
- Polykube operator installed in each member
- sample workload reconciled across members
- routing and status verified without cloud credentials

Current status: the two-cluster lifecycle is scaffolded. Networking and routing validation remain TODO.
