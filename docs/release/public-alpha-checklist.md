# Public Alpha Release Checklist

This checklist must be complete before changing the repository from private to public.

## Required Gate

- [x] Project identity uses Polykube naming, `polykube.dev` API groups, and `polykube-system` namespace.
- [x] Legacy/private reference scan is enforced by `scripts/validate-repo.sh` for known high-confidence patterns.
- [x] Secret and credential scan is enforced by `scripts/validate-repo.sh` for known high-confidence credential patterns.
- [x] Repository includes `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, and `CODE_OF_CONDUCT.md`.
- [x] Third-party dependencies are reviewed through Go module metadata and remain source-only for alpha.
- [x] Known limitations are documented in `docs/known-limitations.md`.
- [x] Local repository validation runs with `bash scripts/validate-repo.sh`.
- [ ] Local multicluster release validation gate passes from a clean machine or disposable VM before public visibility changes.
- [ ] Public release tag and release notes are reviewed.
- [ ] Repository visibility change is approved by the maintainer.

## Audit Evidence

Run this before public release:

```bash
bash scripts/validate-repo.sh
```

The validator checks:

- required public docs and scaffold files exist
- high-confidence legacy/private reference patterns are absent
- high-confidence credential patterns are absent
- trailing whitespace is absent from docs, YAML, and shell scripts
- shell scripts parse with `bash -n`
- Go code is formatted and `go test ./...` passes under `operator/`
- OpenTofu formatting passes when `tofu` is installed
- CRD client dry-run runs when `kubectl` has a configured context
- GitOps operator component renders with `kubectl kustomize` when `kubectl` is installed

## Clean-Machine Multicluster Release Gate

Use a disposable development machine or VM with no repository-local state.

1. Install prerequisites: Git, Docker-compatible runtime, and `mise`.
2. Clone the repository into a fresh directory.
3. Run `mise install`.
4. Run `mise run local:release:validate -- --clusters alpha,beta --workers 0`.
5. Review the evidence log written under `examples/local-multicluster/state/release-evidence/`.
6. Confirm the evidence includes repository validation, cluster status, Cilium and ClusterMesh status, global-service probe responses from both clusters, operator `--cluster-member-name` args, Workload status, Service annotations, cross-cluster HTTP probe, and GitOps render.
7. Record the evidence log path and key command output in the release notes.

The manual equivalent and expected outputs are documented in `docs/release/e2e-validation.md`. Use `examples/local-multicluster/README.md` for demo-oriented operation outside the release gate.

## License And Notices

- Project license is Apache-2.0 in `LICENSE`.
- New source files do not require per-file license headers for alpha unless a generated or copied source requires one.
- Do not copy third-party source into this repository without adding required notices.
- Keep dependency updates reviewable and prefer upstream package managers over vendored code.

## Final Visibility Change

Only change repository visibility after all required gate items are checked and the maintainer confirms the release notes are ready.
