# Known Limitations

Polykube is an experimental alpha. It is not production-ready.

## Operator

- The `Workload` controller currently reconciles local `Deployment` and `Service` objects only.
- `Workload.status.targets[]` reports local-cluster state; cross-cluster aggregation is not implemented.
- The operator does not yet reconcile `ClusterMember`, `Federation`, `ServiceEndpoint`, or `DatastoreBinding` resources.
- Runtime health is inferred from `DeploymentAvailable`; richer workload probes and failure reasons are follow-up work.
- CRD manifests are hand-written alpha bases; generated CRD and deepcopy workflows are follow-up work.

## Local Multicluster Demo

- The local demo validates k0s cluster lifecycle, Cilium/ClusterMesh bootstrap, and global-service routing probes.
- The local demo does not yet install the Polykube operator end-to-end across all members.
- The local demo assumes a Docker-compatible runtime and `mise` tasks.

## Cloud Bootstrap

- OpenTofu code currently emits Polykube manifests from caller-provided cluster outputs.
- The repository does not create cloud networks, managed clusters, IAM, DNS, certificates, or container registries.
- AWS/GCP wiring is an example path, not a required product assumption.
- Provider CNI defaults are not abstracted away. In particular, GKE clusters intended for self-managed Cilium ClusterMesh should be created without Dataplane V2, and underlay route advertisement must be validated independently. See `networking-caveats.md`.

## GitOps

- The GitOps operator component uses the placeholder image `polykube-operator:dev`.
- Consumers must provide their own release image and overlay before cluster installation.
- CRDs must be applied before the operator component.

## Routing And Data

- Progressive rollout, canary, blue/green, and promotion workflows are expected to come from dedicated rollout controllers.
- Production-grade global traffic automation is out of scope for public alpha.
- Direct pod-IP reachability and Cilium global-service translation are separate readiness gates; do not treat one as proof of the other.
- Datastore replication is represented as intent only; no datastore operator integration is implemented.

## Security

- No public supported versions exist yet.
- No formal vulnerability disclosure SLA exists yet.
- Do not use alpha manifests with production credentials or production clusters without independent review.
