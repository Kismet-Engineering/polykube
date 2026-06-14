# Roadmap

## Alpha 0: Repo and Identity

- Create private repository scaffold.
- Establish license, project structure, and GitHub issue workflow.
- Define public scope, non-goals, and sanitization rules.

## Alpha 1: Local Multicluster Demo

- Extract and sanitize local multicluster bootstrap.
- Validate cross-cluster networking and service routing.
- Document a repeatable local demo.

## Alpha 2: Operator Core

- Define CRD model v0.
- Port runtime reconciliation into Kubernetes controllers.
- Report per-cluster rollout status through resource status conditions.

## Alpha 3: Cloud Bootstrap

- Extract and sanitize OpenTofu bootstrap modules.
- Generate cluster membership manifests from bootstrap outputs.
- Add AWS/GCP reference path.

## Alpha 4: Public Release

- Complete public-brand sanitization audit.
- Publish alpha docs and examples.
- Open repository visibility after review.
