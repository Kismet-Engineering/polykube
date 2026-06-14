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

bash -n scripts/validate-repo.sh

printf 'repository validation passed\n'
