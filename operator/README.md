# Operator

This directory contains the Polykube Kubernetes operator and CRD implementation.

The operator is the primary product boundary for the first public alpha.

## Current Scope

- `api/*/v1alpha1`: initial Go type definitions for the accepted v0 CRD model.
- `cmd/polykube-operator`: placeholder binary entrypoint.

Controllers, CRD manifests, generated deepcopy code, and runtime reconciliation are follow-up work.
