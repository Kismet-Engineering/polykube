#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

required_files=(
  README.md
  LICENSE
  SECURITY.md
  CONTRIBUTING.md
  CODE_OF_CONDUCT.md
  docs/architecture.md
  docs/roadmap.md
  docs/decisions/0001-project-identity.md
  docs/decisions/0002-public-alpha-scope.md
  docs/decisions/0003-crd-model-v0.md
  operator/config/crd/bases/data.polykube.dev_datastorebindings.yaml
  operator/config/crd/bases/infrastructure.polykube.dev_clustermembers.yaml
  operator/config/crd/bases/infrastructure.polykube.dev_federations.yaml
  operator/config/crd/bases/routing.polykube.dev_serviceendpoints.yaml
  operator/config/crd/bases/runtime.polykube.dev_workloads.yaml
)

for path in "${required_files[@]}"; do
  if [[ ! -f "$path" ]]; then
    printf 'missing required file: %s\n' "$path" >&2
    exit 1
  fi
done

banned_pattern='zingbang|ZingBang|api\.zingbang|apps\.zingbang|app\.zingbang|admin\.zingbang|op://|1Password|Plane|Resend'
if git grep -n -E "$banned_pattern" -- ':!scripts/validate-repo.sh'; then
  printf 'banned legacy/private reference found\n' >&2
  exit 1
fi

if git grep -n -E '[[:blank:]]$' -- '*.md' '*.yml' '*.yaml' '*.sh'; then
  printf 'trailing whitespace found\n' >&2
  exit 1
fi

while IFS= read -r script_path; do
  bash -n "${script_path}"
done < <(git ls-files '*.sh')

if [[ -f operator/go.mod ]]; then
  if ! command -v go >/dev/null 2>&1; then
    printf 'missing required command: go\n' >&2
    exit 1
  fi

  gofmt_output="$(gofmt -l operator)"
  if [[ -n "${gofmt_output}" ]]; then
    printf 'gofmt required for:\n%s\n' "${gofmt_output}" >&2
    exit 1
  fi

  (cd operator && go test ./...)
fi

if command -v kubectl >/dev/null 2>&1; then
  kubectl apply --dry-run=client --validate=false -f operator/config/crd/bases >/dev/null
fi

printf 'repository validation passed\n'
