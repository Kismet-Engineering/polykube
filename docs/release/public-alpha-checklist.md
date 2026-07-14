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
- [ ] Quickstart is validated from a clean machine or disposable VM before public visibility changes.
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

## Clean-Machine Quickstart Gate

Use a disposable development machine or VM with no repository-local state.

1. Install prerequisites: Git, Go matching `operator/go.mod`, `kubectl`, Docker-compatible runtime, and `mise`.
2. Clone the repository into a fresh directory.
3. Run `bash scripts/validate-repo.sh`.
4. Follow `examples/local-multicluster/README.md` through cluster creation, Cilium install, ClusterMesh connect, verify, and global-service probe.
5. Render `gitops/components/operator` with `kubectl kustomize gitops/components/operator`.
6. If `tofu` is installed, run `tofu fmt -check -recursive infra/tofu`.
7. Record command output in the release notes.

## License And Notices

- Project license is Apache-2.0 in `LICENSE`.
- New source files do not require per-file license headers for alpha unless a generated or copied source requires one.
- Do not copy third-party source into this repository without adding required notices.
- Keep dependency updates reviewable and prefer upstream package managers over vendored code.

## Final Visibility Change

Only change repository visibility after all required gate items are checked and the maintainer confirms the release notes are ready.
