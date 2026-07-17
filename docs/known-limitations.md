# Known Limitations

Polykube is an experimental alpha. It is not production-ready.

## Operator

- All five alpha controllers are implemented and registered: `ClusterMember`, `Federation`, `Workload`, `ServiceEndpoint`, and `DatastoreBinding`.
- The `Workload` controller reconciles local `Deployment` and `Service` objects only.
- `Workload.status.targets[]` reports local-cluster state. The read-only `polykube-status` CLI aggregates it on demand from explicitly selected kubeconfig contexts; there is no continuously running status service or persisted aggregate state.
- `ClusterMember` and `Federation` reconcile identity, membership, and readiness status; they do not configure cloud infrastructure or networking.
- `ServiceEndpoint` applies Cilium global-service annotations to the generated `Service`; Gateway API fields are accepted but not acted on yet.
- `DatastoreBinding` injects connection env vars into the generated `Deployment`; it does not provision databases, configure replication, or enforce `conflictPolicy`.
- Controllers do not adopt same-name `Deployment` or `Service` objects. Ownership conflicts are reported as degraded status and must be resolved by renaming or removing the conflicting object.
- Missing dependencies and ownership conflicts use periodic retries rather than dedicated watches for every referenced resource.
- Runtime health is inferred from `DeploymentAvailable`; richer workload probes and failure reasons are follow-up work.
- CRD manifests and deepcopy code are generated from Go API types; API changes still require alpha-stage review before release.

## Local Multicluster Demo

- The local demo validates k0s cluster lifecycle, Cilium/ClusterMesh bootstrap, operator deployment, sample Workload reconciliation, ServiceEndpoint annotations, and global-service routing probes.
- The local release gate is automated for local k0s/Cilium validation, but it still depends on local Docker runtime health and does not replace live cloud validation.
- The local demo assumes a Docker-compatible runtime and `mise` tasks.

## Cloud Bootstrap

- OpenTofu code currently emits Polykube manifests from caller-provided cluster outputs.
- The repository does not create cloud networks, managed clusters, IAM, DNS, certificates, or container registries.
- AWS/GCP wiring is an example path, not a required product assumption.
- Provider CNI defaults are not abstracted away. In particular, GKE clusters intended for self-managed Cilium ClusterMesh should be created without Dataplane V2, and underlay route advertisement must be validated independently. See `networking-caveats.md`.

## GitOps

- The GitOps operator component defaults to the published alpha operator image for live-cloud installs.
- Live-cloud consumers should pin a reviewed published image tag in an overlay before promotion.
- Local demos use the separate `polykube-operator:dev` image path and local deployment scripts.
- CRDs must be applied before the operator component.

## Routing And Data

- Progressive rollout, canary, blue/green, and promotion workflows are expected to come from dedicated rollout controllers.
- Production-grade global traffic automation is out of scope for public alpha.
- Direct pod-IP reachability and Cilium global-service translation are separate readiness gates; do not treat one as proof of the other.
- Datastore replication is represented as intent only; no datastore operator integration, database provisioning, replication setup, or conflict-policy enforcement is implemented.

## Security

- No public supported versions exist yet.
- No formal vulnerability disclosure SLA exists yet.
- The default operator component watches all namespaces and can read all Secrets and ConfigMaps and manage owned Deployments and Services cluster-wide. A single-workload-namespace profile is available under `gitops/overlays/operator-namespace-scoped`; see `docs/security.md`.
- The namespace-scoped profile does not support cross-namespace references or multiple watched workload namespaces. Multiple restricted operator instances in one cluster are not tested.
- `DatastoreBinding` writes the selected connection URL into a Deployment environment value, so readers of that Deployment can see the URL.
- Do not use alpha manifests with production credentials or production clusters without independent review.
