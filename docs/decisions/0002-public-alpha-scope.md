# Decision 0002: Public Alpha Scope

Status: accepted

## Context

Polykube is being created as a clean open source project from prior multicloud backend infrastructure and control-plane experiments. The first risk is importing too much product-specific code before the public boundary is clear.

The public alpha should prove the reusable pattern, not preserve every historical feature.

## Decision

The first public alpha will focus on a Kubernetes-native, self-hostable path for portable backend workloads across multiple clusters.

The alpha scope is:

- CRDs for cluster membership, federation intent, workload intent, routing intent, and optional datastore binding intent.
- A local-cluster operator that reconciles workload runtime resources using only local cluster credentials.
- A local multicluster demo that validates the minimum cross-cluster routing path without cloud credentials.
- Sanitized infrastructure bootstrap examples that can emit reviewable Polykube manifests.
- Sanitized GitOps component examples for runtime dependencies.
- Documentation for architecture, tradeoffs, threat boundaries, and known limitations.

The alpha must be installable and understandable without a hosted control plane, private cloud account, private domain, private container registry, private credential manager, or managed service subscription.

## Non-Goals

The first public alpha will not include:

- Managed SaaS workflows.
- Hosted admin or customer dashboards.
- Billing, usage metering for commercial billing, subscription, or account lifecycle features.
- Customer onboarding email flows or hosted credential delivery.
- A central service that needs credentials for every member cluster.
- Production-grade global traffic automation.
- A general cloud resource management plane.
- A hard dependency on one CNI, DNS provider, certificate manager, cloud provider, datastore, or secret manager.

Reference examples may use specific tools where needed, but those tools must be documented as examples rather than hidden product assumptions.

## Extraction Rules

Implementation copied into this repository must satisfy these rules before commit:

- No legacy project names, hosted domains, or business-specific identifiers.
- No private credential references, secret manager item paths, live account IDs, or private repository URLs.
- No default behavior that mutates live cloud or cluster resources without an explicit command and documentation.
- No user-facing dependency on a hosted API server unless the operator path works independently.
- Kubernetes labels, annotations, API groups, namespaces, image names, and binary names must use Polykube identity.
- Provider-specific code must be isolated behind examples, modules, adapters, or documented interfaces.
- Tests and fixtures must use neutral names and synthetic identifiers.

## Initial Resource Boundary

The first CRD design pass should start from these resources:

- `ClusterMember`: one participating cluster with provider, region, zone, endpoint, and reachability metadata.
- `Federation`: a set of cluster members plus policy for participation and routing posture.
- `Workload`: desired runtime workload intent, including image, environment, and target policy.
- `DeploymentTarget`: per-cluster rollout state, represented either as a child resource or status subresource depending on the CRD design.
- `ServiceEndpoint`: route and failover intent for exposing a workload.
- `DatastoreBinding`: optional data dependency intent for replicated datastore examples.

## Consequences

- The operator is the primary product boundary for alpha.
- The previous HTTP API concepts may inform the CRD model, but an HTTP API is not required for alpha.
- Local demo work should come before cloud bootstrap polish.
- Public release readiness depends on a sanitization audit, not just working code.
