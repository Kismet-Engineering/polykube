# API Field Support

Every `v1alpha1` spec field is accepted by the CRD schema. Not every field changes runtime behavior yet. This page classifies each field so submitted intent is not mistaken for enforced behavior.

- **Reconciled:** the operator reads the field, and it changes runtime objects or status.
- **Metadata-only:** descriptive information for people and tooling. The operator does not act on it, and there is no plan for it to.
- **Future integration:** accepted intent the operator does not act on yet. Setting it has no runtime effect in this alpha. Implementation is tracked in [#45](https://github.com/Kismet-Engineering/polykube/issues/45) and waits on a multicluster test harness.

The CRD descriptions (`kubectl explain <kind>.spec.<field>`) state the same behavior for every metadata-only and future-integration field.

## Status reporting

When a user sets a future-integration field, the owning controller sets an informational condition:

| Condition | Status | Reason | Meaning |
| --- | --- | --- | --- |
| `FieldsNotEnforced` | `True` | `AcceptedNotEnforced` | The message names each set field that is accepted but not acted on. |

The condition does not affect `Ready`, `Degraded`, `Available`, or `Pending`, and it needs no recovery action. It is removed once the fields are cleared. Metadata-only fields never produce the condition, because they are descriptive by design and bootstrap tooling sets them routinely.

## ClusterMember

| Field | Class | Behavior |
| --- | --- | --- |
| `provider`, `region`, `clusterName` | Reconciled | Required. The controller validates them and reports `Ready`. |
| `zone`, `environment` | Metadata-only | Descriptive placement context. |
| `apiEndpoint` | Metadata-only | Never contacted. Each operator reconciles only its local cluster. |
| `podCIDR`, `serviceCIDR` | Metadata-only | Networking is configured outside Polykube. See [`networking-caveats.md`](networking-caveats.md). |
| `labels` | Metadata-only | Federation and Workload selectors match `metadata.labels`, not `spec.labels`. Put selection labels in `metadata.labels`. |

## Federation

| Field | Class | Behavior |
| --- | --- | --- |
| `members`, `memberSelector` | Reconciled | Resolve membership for `status.members`, `readyMembers`, and Workload placement. |
| `routingMode` | Metadata-only | Describes the intended routing posture. Routing is reconciled from `ServiceEndpoint.spec.routingMode`. |
| `networking` | Metadata-only | Describes the networking substrate. It configures nothing. |
| `defaultTargetPolicy` | Future integration | Not used for placement. Set `Workload.spec.targetPolicy` instead. |

## Workload

| Field | Class | Behavior |
| --- | --- | --- |
| `federationRef.name` | Reconciled | Federation membership gates local reconciliation. |
| `federationRef.namespace` | Metadata-only | Ignored because `Federation` is cluster-scoped. |
| `image`, `replicas`, `ports`, `env`, `envFrom`, `imagePullSecrets`, `serviceAccountName` | Reconciled | Rendered into the local `Deployment`, plus a `Service` when ports are declared. |
| `targetPolicy.members`, `targetPolicy.memberSelector` | Reconciled | Select which members run the Workload. Excluded members report `Pending`. |
| `targetPolicy.strategy` | Future integration | No placement or rollout strategy is applied. |
| `rolloutRef` | Future integration | No external rollout controller is consulted. See [decision 0003](decisions/0003-crd-model-v0.md). |

## ServiceEndpoint

| Field | Class | Behavior |
| --- | --- | --- |
| `workloadRef`, `routingMode`, `primaryMemberRef` | Reconciled | Apply Cilium global-service annotations to the Workload's controlled `Service`. |
| `hostnames` | Metadata-only | Copied to `status.resolvedHostnames`. No DNS, certificate, or ingress configuration. |
| `failoverPolicy` | Future integration | No health-driven failover. Active/passive routing follows `primaryMemberRef` only. |
| `gatewayRef` | Future integration | No Gateway API resources are created or attached. |

## DatastoreBinding

| Field | Class | Behavior |
| --- | --- | --- |
| `workloadRef`, `connectionRef` | Reconciled | Resolve the Workload and local connection `Secret`, then inject env vars into the controlled `Deployment`. |
| `engine` | Reconciled | Validated against the supported engines. It selects no engine-specific behavior. |
| `replicationMode` | Reconciled | Exposed to the application as `DATASTORE_<NAME>_REPLICATION_MODE`. No datastore replication is configured. |
| `conflictPolicy` | Future integration | Not enforced or passed to the application. |

## Examples

The sample manifests under `examples/local-multicluster/manifests` and the manifests rendered by `infra/tofu` set no future-integration fields. The Tofu module sets metadata-only fields such as `routingMode`, `networking`, `zone`, `environment`, and the CIDRs for documentation and review purposes.
