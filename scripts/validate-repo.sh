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
  docs/known-limitations.md
  docs/release/operator-images.md
  docs/decisions/0001-project-identity.md
  docs/decisions/0002-public-alpha-scope.md
  docs/decisions/0003-crd-model-v0.md
  docs/release/public-alpha-checklist.md
  gitops/components/operator/README.md
  gitops/components/operator/kustomization.yaml
  gitops/components/operator/namespace.yaml
  gitops/components/operator/service-account.yaml
  gitops/components/operator/cluster-role.yaml
  gitops/components/operator/cluster-role-binding.yaml
  gitops/components/operator/deployment.yaml
  infra/tofu/modules/polykube-manifests/README.md
  infra/tofu/modules/polykube-manifests/versions.tf
  infra/tofu/modules/polykube-manifests/variables.tf
  infra/tofu/modules/polykube-manifests/main.tf
  infra/tofu/modules/polykube-manifests/outputs.tf
  infra/tofu/examples/aws-gcp/README.md
  infra/tofu/examples/aws-gcp/versions.tf
  infra/tofu/examples/aws-gcp/variables.tf
  infra/tofu/examples/aws-gcp/main.tf
  infra/tofu/examples/aws-gcp/outputs.tf
  operator/Dockerfile
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

if git grep -n -E '[[:blank:]]$' -- '*.md' '*.yml' '*.yaml' '*.sh'; then
  printf 'trailing whitespace found\n' >&2
  exit 1
fi

while IFS= read -r script_path; do
  bash -n "${script_path}"
done < <(git ls-files '*.sh')

sanitize_patterns=(
  'AKIA[0-9A-Z]{16}'
  'ASIA[0-9A-Z]{16}'
  'AIza[0-9A-Za-z_-]{35}'
  'gh[pousr]_[0-9A-Za-z_]{20,}'
  'github_pat_[0-9A-Za-z_]{20,}'
  'xox[baprs]-[0-9A-Za-z-]{10,}'
  '-----BEGIN (RSA |DSA |EC |OPENSSH |PGP )?PRIVATE KEY-----'
  '"private_key"[[:space:]]*:[[:space:]]*"-----BEGIN PRIVATE KEY-----'
  '"private_key_id"[[:space:]]*:[[:space:]]*"[0-9A-Fa-f]{32,}"'
  '"client_email"[[:space:]]*:[[:space:]]*"[^"]+\.iam\.gserviceaccount\.com"'
  'git@(github|gitlab|bitbucket)\.com:[^[:space:]]*(private|internal|legacy)[^[:space:]]*'
  'https://(github|gitlab|bitbucket)\.com/[^[:space:]]*(private|internal|legacy)[^[:space:]]*'
)

for pattern in "${sanitize_patterns[@]}"; do
  set +e
  grep_output="$(git grep -n -I -E -e "${pattern}" -- . ':!scripts/validate-repo.sh' 2>&1)"
  grep_status="$?"
  set -e

  if [[ "${grep_status}" -eq 0 ]]; then
    printf '%s\n' "${grep_output}"
    printf 'sanitization pattern matched: %s\n' "${pattern}" >&2
    exit 1
  fi

  if [[ "${grep_status}" -ne 1 ]]; then
    printf 'sanitization scan failed for pattern: %s\n' "${pattern}" >&2
    printf '%s\n' "${grep_output}" >&2
    exit 1
  fi
done

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

if command -v tofu >/dev/null 2>&1; then
  tofu fmt -check -recursive infra/tofu
fi

if command -v kubectl >/dev/null 2>&1 && kubectl config current-context >/dev/null 2>&1; then
  kubectl apply --dry-run=client --validate=false -f operator/config/crd/bases >/dev/null
fi

if command -v kubectl >/dev/null 2>&1; then
  kubectl kustomize gitops/components/operator >/dev/null
fi

printf 'repository validation passed\n'
