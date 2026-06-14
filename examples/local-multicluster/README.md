# Local Multicluster Example

This example is the first repeatable Polykube validation path. It currently supports local k0s cluster lifecycle plus Cilium/ClusterMesh bootstrap. Workload routing probes will be added in a follow-up pass.

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

## Install Cilium And ClusterMesh

```bash
mise run local:cilium:preflight -- --clusters alpha,beta
mise run local:cilium:install -- --clusters alpha,beta
mise run local:cilium:clustermesh:enable -- --clusters alpha,beta --service-type NodePort
mise run local:cilium:clustermesh:connect -- --source alpha --destination beta
mise run local:cilium:verify -- --source alpha --destination beta
```

Inspect or reset Cilium state:

```bash
mise run local:cilium:status -- --clusters alpha,beta
mise run local:cilium:reset -- --clusters alpha,beta
```

## Target Proof

- two local Kubernetes clusters
- cross-cluster networking substrate
- Polykube operator installed in each member
- sample workload reconciled across members
- routing and status verified without cloud credentials

Current status: the two-cluster lifecycle and Cilium/ClusterMesh bootstrap are scaffolded. Workload routing validation remains TODO.
