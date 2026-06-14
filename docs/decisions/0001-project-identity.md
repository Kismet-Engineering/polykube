# Decision 0001: Project Identity

Status: accepted

## Decision

The project name is Polykube.

The repository starts private under `Kismet-Engineering/polykube` and will be prepared for a future public alpha release under the Apache-2.0 license.

The Kubernetes API group root is `polykube.dev`.

## Rationale

Polykube communicates Kubernetes-native multicluster portability without tying the project to a hosted SaaS product or a specific cloud provider.

## Consequences

- Public-facing code and docs must avoid prior project branding, hosted domains, private credential references, or business-specific assumptions.
- Copied implementation code must be sanitized before landing in this repository.
