# Getting Started

Polykube is not ready for users yet. This guide will become the shortest local path to a working multicluster demo.

## Target Alpha Flow

1. Create two local Kubernetes clusters.
2. Install the networking substrate.
3. Install the Polykube operator.
4. Apply `ClusterMember`, `Federation`, and `Workload` resources.
5. Verify cross-cluster service routing and workload status.

The first implementation pass will make this flow concrete under `examples/local-multicluster/`.
