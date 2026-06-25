# Decision 0003: CRD Model v0

Status: accepted

## Context

Polykube needs a small Kubernetes API surface before operator code is imported or written. The v0 model should express the minimum useful multicluster backend pattern while avoiding a large platform API clone.

The operator-first boundary from Decision 0002 means desired state must be representable as Kubernetes resources and reconcilable without a hosted control plane.

## Decision

Polykube v0 will start with five CRDs:

- `ClusterMember.infrastructure.polykube.dev`
- `Federation.infrastructure.polykube.dev`
- `Workload.runtime.polykube.dev`
- `ServiceEndpoint.routing.polykube.dev`
- `DatastoreBinding.data.polykube.dev`

Per-cluster deployment target state will be represented under `Workload.status.targets[]` in v0 instead of as a separate `DeploymentTarget` CRD.

Progressive rollout mechanics are not part of the core v0 API. Polykube reconciles placement and runtime wiring for the desired workload state; canaries, blue/green promotion, traffic-shift gates, approval workflows, and rich rollout history should be handled by dedicated rollout systems or future integrations.

## Resource Model

### ClusterMember

`ClusterMember` describes one participating Kubernetes cluster.

Scope: cluster-scoped.

Spec fields:

- `provider`: provider identifier such as `aws`, `gcp`, `azure`, `kind`, or `other`.
- `region`: provider region or locality.
- `zone`: optional zone for zonal clusters.
- `environment`: logical environment label such as `dev`, `staging`, or `prod`.
- `clusterName`: human-readable cluster name.
- `apiEndpoint`: optional Kubernetes API endpoint metadata.
- `podCIDR`: optional pod CIDR for diagnostics and generated examples.
- `serviceCIDR`: optional service CIDR for diagnostics and generated examples.
- `labels`: freeform selection metadata.

Status fields:

- `conditions`: standard Kubernetes conditions.
- `observedGeneration`: last reconciled generation.
- `lastObservedAt`: timestamp of last member observation.

### Federation

`Federation` describes a set of participating cluster members and the default multicluster posture.

Scope: cluster-scoped.

Spec fields:

- `memberSelector`: label selector for `ClusterMember` resources.
- `members`: optional explicit member references.
- `routingMode`: default routing posture, initially `ActivePassive` or `ActiveActive`.
- `defaultTargetPolicy`: default workload placement policy.
- `networking`: optional declared networking substrate metadata for examples and diagnostics.

Status fields:

- `conditions`: standard Kubernetes conditions.
- `members`: resolved member summary.
- `readyMembers`: count of ready members.

### Workload

`Workload` describes desired runtime workload intent.

Scope: namespaced.

Spec fields:

- `federationRef`: target federation reference.
- `image`: container image reference.
- `imagePullSecrets`: optional pull secret references.
- `ports`: exposed container ports.
- `env`: literal environment variables.
- `envFrom`: Kubernetes-style config and secret references.
- `targetPolicy`: member selection, explicit member list, or placement strategy.
- `rolloutRef`: optional reference to an external rollout controller or strategy resource.
- `replicas`: per-member replica count for v0.
- `serviceAccountName`: optional service account override.

Status fields:

- `conditions`: standard Kubernetes conditions.
- `observedGeneration`: last reconciled generation.
- `targets`: per-member rollout status entries.
- `activeImage`: image observed by the controller.

`Workload.status.targets[]` entries contain:

- `clusterMemberRef`
- `state`: `Pending`, `Reconciling`, `Available`, `Degraded`, or `Failed`.
- `runtimeRef`: reconciled runtime object reference.
- `lastTransitionTime`
- `message`

### ServiceEndpoint

`ServiceEndpoint` describes routing and failover intent for a workload.

Scope: namespaced.

Spec fields:

- `workloadRef`: target workload reference.
- `hostnames`: requested hostnames.
- `routingMode`: `ActivePassive` or `ActiveActive`.
- `primaryMemberRef`: preferred primary member for active/passive routing.
- `failoverPolicy`: explicit failover behavior and health threshold metadata.
- `gatewayRef`: optional Gateway API reference.

Status fields:

- `conditions`: standard Kubernetes conditions.
- `activeMemberRef`: currently active member for active/passive routing.
- `resolvedHostnames`: hostnames observed or emitted by the controller.
- `lastTransitionTime`

### DatastoreBinding

`DatastoreBinding` describes optional data dependency and replication intent for examples and future integrations.

Scope: namespaced.

Spec fields:

- `workloadRef`: target workload reference.
- `engine`: datastore engine identifier.
- `connectionRef`: Kubernetes secret reference or provider-specific reference.
- `replicationMode`: `None`, `ActivePassive`, or `ActiveActive`.
- `conflictPolicy`: `Reject`, `LastWriteWins`, or `External`.

Status fields:

- `conditions`: standard Kubernetes conditions.
- `observedGeneration`: last reconciled generation.
- `message`: integration-specific status summary.

## State and Conditions

All resources will use Kubernetes-style conditions with at least:

- `Ready`
- `Reconciling`
- `Degraded`

Controllers should prefer condition updates and events over hidden internal state. Any durable state needed for reconciliation should be derivable from Kubernetes resources or explicitly modeled in status.

## Ownership Rules

- `ClusterMember` and `Federation` are infrastructure intent.
- `Workload` owns generated per-member runtime resources such as Deployments, Services, runtime Secrets, and optional HTTPRoutes unless an integration-specific owner is documented.
- `ServiceEndpoint` owns routing policy resources where Polykube emits them.
- `DatastoreBinding` is advisory in v0 unless an example integration explicitly reconciles it.
- Controllers must not require credentials for remote member clusters. Each cluster reconciles its local slice of desired state.
- Progressive delivery systems may own the rollout strategy for generated or referenced runtime workloads. In that case, Polykube should avoid fighting those controllers and should report observed target health rather than duplicating their rollout state machines.

## Rejected Alternatives

### Separate DeploymentTarget CRD in v0

Rejected for now. A separate child resource may be useful later, but v0 can represent per-member rollout state in `Workload.status.targets[]`. This keeps the first public API smaller and avoids inventing lifecycle semantics before the operator implementation proves the need.

### Central API-Backed Queue

Rejected for alpha. A central queue recreates hosted control-plane assumptions and requires coordination outside Kubernetes. The v0 model should reconcile from Kubernetes desired state.

### Built-In Progressive Rollout Engine

Rejected for v0. Canary analysis, blue/green promotion, approvals, and deployment history are mature concerns with dedicated Kubernetes-native tools. Polykube should interoperate with those systems instead of becoming a second rollout engine.

### Provider-Specific Top-Level CRDs

Rejected for v0. Provider-specific bootstrap belongs in examples or infrastructure modules. The operator API should stay provider-neutral.

## Consequences

- The first operator implementation can focus on `ClusterMember`, `Federation`, and `Workload` before routing and datastore integrations are complete.
- `DeploymentTarget` can be introduced later if status arrays become insufficient.
- Rollout integration can start as references and ownership boundaries before any deep controller coupling is added.
- CRD YAML and generated Go types should use `polykube.dev` API groups from the first implementation pass.
