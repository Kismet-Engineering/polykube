# Architecture

Polykube separates cloud bootstrap, cluster membership, workload reconciliation, and routing policy.

## System Overview

```mermaid
flowchart LR
    Infra[Cloud or local bootstrap] --> Artifacts[Reviewable Kubernetes manifests]
    Artifacts --> GitOps[GitOps source of truth]

    subgraph ClusterA[Cluster A]
        AAPI[Kubernetes API]
        AOperator[Polykube operator]
        ARuntime[Deployments, Services, routing objects]
        AAPI --> AOperator
        AOperator --> ARuntime
    end

    subgraph ClusterB[Cluster B]
        BAPI[Kubernetes API]
        BOperator[Polykube operator]
        BRuntime[Deployments, Services, routing objects]
        BAPI --> BOperator
        BOperator --> BRuntime
    end

    GitOps --> AAPI
    GitOps --> BAPI
    ARuntime <--> Network[Multicluster networking substrate]
    BRuntime <--> Network
```

Polykube keeps the durable control surface in Kubernetes manifests and status. Bootstrap tooling can help create manifests, but the operator boundary stays inside each participating cluster.

## Control Model

The operator is the primary control plane. Desired state should live in Kubernetes resources and be reconcilable through GitOps.

Initial API groups use the `polykube.dev` root:

- `infrastructure.polykube.dev`: cluster membership and federation substrate.
- `runtime.polykube.dev`: workloads and rollout targets.
- `routing.polykube.dev`: service endpoints and routing policy.
- `data.polykube.dev`: datastore bindings and replication intent.

The v0 CRD model is defined in `docs/decisions/0003-crd-model-v0.md`.

```mermaid
flowchart TB
    Federation[Federation] --> ClusterMember[ClusterMember]
    Workload[Workload] --> Federation
    ServiceEndpoint[ServiceEndpoint] --> Workload
    DatastoreBinding[DatastoreBinding] --> Workload

    Workload --> RuntimeObjects[Generated runtime objects]
    RuntimeObjects --> Deployment[Deployment]
    RuntimeObjects --> Service[Service]
    RuntimeObjects --> Route[Optional routing resources]

    Workload --> Status[Workload.status.targets]
    Status --> TargetA[Cluster A target status]
    Status --> TargetB[Cluster B target status]
```

## Bootstrap Model

Infrastructure bootstrap tools should produce deterministic artifacts that can be reviewed and applied as Kubernetes manifests.

```mermaid
sequenceDiagram
    participant Operator as Human operator
    participant Tofu as OpenTofu examples
    participant Manifests as Polykube manifests
    participant Git as GitOps repo
    participant Cluster as Member clusters

    Operator->>Tofu: provide or discover cluster outputs
    Tofu->>Manifests: render ClusterMember and Federation YAML
    Operator->>Manifests: review generated intent
    Operator->>Git: commit approved manifests and overlays
    Git->>Cluster: reconcile CRDs and runtime components
```

## Runtime Model

Each participating cluster runs local reconciliation with only the credentials needed for that cluster. Multicluster rollout state is aggregated from per-cluster target status rather than a central process holding all cluster credentials.

For v0, per-cluster rollout state lives under `Workload.status.targets[]`. A separate deployment target resource can be introduced later if implementation evidence shows the status array is insufficient.

Polykube does not aim to be a progressive rollout engine. It should interoperate with dedicated rollout controllers for canaries, blue/green promotion, approvals, and traffic-shift gates while retaining responsibility for multicluster placement and runtime wiring.

```mermaid
flowchart LR
    Desired[Workload desired state] --> API[Kubernetes API]

    subgraph MemberCluster[One member cluster]
        API --> Controller[Polykube controller]
        Controller --> LocalDecision[Select local slice of target policy]
        LocalDecision --> Runtime[Apply local runtime resources]
        Runtime --> Observe[Observe local health]
        Observe --> Status[Update local status and conditions]
    end

    Status --> Aggregate[Multicluster status view]
```
