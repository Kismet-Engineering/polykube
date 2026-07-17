# Local Multicluster Demo

## What this builds

Two k0s Kubernetes clusters running on your local machine, connected by Cilium ClusterMesh so pods in each cluster can reach pods in the other. The Polykube operator is deployed into both clusters. This gives you a complete local replica of the multicluster topology — no cloud account, no external dependencies.

## Prerequisites

- Docker-compatible runtime
- `kubectl`
- `mise` for task execution
- `colima` on macOS when using Colima as the Docker runtime

### Colima inotify capacity

The local testbed runs Kubernetes components, Cilium, and ClusterMesh in containers that share the Colima VM's inotify-instance quota. Colima's default `fs.inotify.max_user_instances=128` can be exhausted when two Polykube clusters or other local Kubernetes clusters are running. k0s then reports `Failed to create watcher: too many open files` and never registers its node even when the container's `nofile` limit is sufficient.

Set the current VM limit to at least 512:

```bash
colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=512
```

To persist the setting across Colima restarts, add this system provision script to `~/.colima/default/colima.yaml`, then restart Colima:

```yaml
provision:
  - mode: system
    script: |
      sysctl -w fs.inotify.max_user_instances=512
```

```bash
colima restart
```

Cluster creation checks this value before starting k0s and prints the same remediation when it is too low. This is an inotify-instance limit, not an inode or ordinary open-file limit.

## Release Validation Gate

For release validation, run the repeatable two-cluster gate instead of manually stepping through the demo:

```bash
mise run local:release:validate -- --clusters alpha,beta --workers 0
```

The gate exits nonzero on failure and records evidence under:

```text
examples/local-multicluster/state/release-evidence/
```

It validates cluster creation, Cilium ClusterMesh, global-service routing, operator identity via `--cluster-member-name`, target selection, sample Workload reconciliation, ServiceEndpoint annotations, DatastoreBinding injection and recovery, a cross-cluster HTTP probe, and GitOps operator rendering. The release checklist and expected evidence are documented in `docs/release/e2e-validation.md`.

## Create Clusters

Create two local clusters named `alpha` and `beta`:

```bash
mise run local:cluster:create -- --clusters alpha,beta --workers 0
mise run local:cluster:status
```

Kubeconfigs are written under:

```text
examples/local-multicluster/state/kubeconfigs/
```

Point `kubectl` at both clusters:

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

Install Cilium into each cluster and connect them so cross-cluster pod traffic works:

```bash
mise run local:cilium:preflight -- --clusters alpha,beta
mise run local:cilium:install -- --clusters alpha,beta
mise run local:cilium:clustermesh:enable -- --clusters alpha,beta --service-type NodePort
mise run local:cilium:clustermesh:connect -- --source alpha --destination beta
mise run local:cilium:verify -- --source alpha --destination beta
mise run local:cilium:global-service:probe -- --source alpha --destination beta
```

Inspect or reset Cilium state:

```bash
mise run local:cilium:status -- --clusters alpha,beta
mise run local:cilium:reset -- --clusters alpha,beta
```

## Deploy The Operator

Build the operator image and deploy it to both clusters. Each instance is told its own identity via `--cluster-member-name`:

```bash
mise run operator:image:build -- --image polykube-operator:dev
mise run local:operator:image:load -- --clusters alpha,beta --image polykube-operator:dev
mise run local:operator:deploy -- --clusters alpha,beta --image polykube-operator:dev
```

Verify both operator pods are running:

```bash
kubectl --context polykube-alpha -n polykube-system get pods
kubectl --context polykube-beta  -n polykube-system get pods
```

## Apply The Sample Workload

Apply the sample manifests to both clusters in one step:

```bash
mise run local:demo:apply
```

This applies to each cluster:
- `clustermember-{alpha,beta}.yaml` — declares the local cluster's identity
- `federation.yaml` — groups both members into a single federation
- `workload-echo.yaml` — a simple echo HTTP server
- `serviceendpoint-echo.yaml` — configures ActiveActive Cilium global service routing

## Verify The Demo

Check that ClusterMember resources reached `Ready=True` on each cluster:

```bash
kubectl --context polykube-alpha get clustermember alpha -o yaml | grep -A5 conditions
kubectl --context polykube-beta  get clustermember beta  -o yaml | grep -A5 conditions
```

Check that the Federation resolved both members:

```bash
kubectl --context polykube-alpha get federation local-dev -o yaml | grep -E 'readyMembers|members'
```

Check that the Workload is reconciled on each cluster:

```bash
kubectl --context polykube-alpha -n default get workload echo -o yaml | grep -A5 targets
kubectl --context polykube-beta  -n default get workload echo -o yaml | grep -A5 targets
```

Both should show `state: Reconciling` (or `Available` once pods are running).

Check that the echo Service received Cilium global service annotations:

```bash
kubectl --context polykube-alpha -n default get svc echo -o yaml | grep cilium
kubectl --context polykube-beta  -n default get svc echo -o yaml | grep cilium
```

Both should show `service.cilium.io/global: "true"` and `service.cilium.io/shared: "true"`.

Probe cross-cluster routing by sending a request from alpha to the echo service and checking it can be served by a pod on beta:

```bash
kubectl --context polykube-alpha -n default run probe --rm -i --restart=Never \
  --image=curlimages/curl -- curl -s http://echo:5678
```

## What success looks like

- Two local Kubernetes clusters with reachable API endpoints
- Cross-cluster pod networking via Cilium ClusterMesh
- Polykube operator running in each cluster with correct `--cluster-member-name`
- `ClusterMember` resources at `Ready=True` on each cluster
- `Federation local-dev` shows `readyMembers: 2`
- `Workload echo` shows `status.targets[0].state: Available` on each cluster
- `Service echo` has `service.cilium.io/global: "true"` on both clusters
- A pod in cluster alpha can reach the echo service and receive a response
