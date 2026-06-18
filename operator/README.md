# Operator

This directory contains the Polykube Kubernetes operator and CRD implementation.

The operator is the primary product boundary for the first public alpha.

## Current Scope

- `api/*/v1alpha1`: initial Go type definitions for the accepted v0 CRD model.
- `config/crd/bases`: hand-written v1alpha1 CRD manifests for the alpha API surface.
- `internal/controller`: controller scaffolds for Polykube resources.
- `internal/scheme`: shared Kubernetes scheme registration for all Polykube API groups.
- `cmd/polykube-operator`: controller-runtime manager entrypoint with health and readiness probes.

The current `Workload` controller observes resources but does not create runtime objects yet. Generated deepcopy code and runtime reconciliation are follow-up work.
