#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd docker
require_cmd kubectl

echo "## Docker Runtime"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | sed '1!{/^polykube-/!d;}'

echo
echo "## Kubeconfigs"
if [[ ! -d "${KUBECONFIG_DIR}" ]]; then
  echo "No kubeconfigs found."
  exit 0
fi

shopt -s nullglob
for kubeconfig in "${KUBECONFIG_DIR}"/*.yaml; do
  context="$(kubectl --kubeconfig "${kubeconfig}" config current-context 2>/dev/null || true)"
  echo "kubeconfig=${kubeconfig} context=${context}"
  if [[ -n "${context}" ]]; then
    if ! kubectl --kubeconfig "${kubeconfig}" --context "${context}" get nodes -o wide; then
      echo "warning: unable to query nodes for ${context}" >&2
    fi
  fi
  echo
done

echo "## Notes"
echo "- A fresh k0s cluster can show NotReady until a CNI is installed."
echo "- Kubeconfigs are stored under examples/local-multicluster/state/kubeconfigs/."
