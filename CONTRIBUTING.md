# Contributing

Polykube is currently in experimental public alpha. Contributions should keep the repository neutral, portable, and free of project-specific credentials, domains, or business assumptions.

## Ground Rules

- Keep changes small and reviewable.
- Prefer Kubernetes-native contracts over bespoke control-plane state.
- Do not add secrets, private endpoints, private domains, or organization-specific credential references.
- Document operational tradeoffs directly.
- Add validation steps to pull requests whenever possible.

## Development Flow

1. Open or claim a GitHub issue.
2. Keep the branch scoped to one concern.
3. Add or update docs when behavior or architecture changes.
4. Run relevant validation before opening a pull request.

## Project Areas

- `operator`: Kubernetes controllers, CRDs, and runtime reconciliation.
- `infra`: infrastructure bootstrap modules and examples.
- `gitops`: reusable in-cluster runtime components.
- `examples`: local and cloud validation paths.
- `docs`: architecture, decisions, and user-facing guides.
