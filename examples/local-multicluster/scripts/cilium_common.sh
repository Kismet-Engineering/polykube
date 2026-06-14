#!/usr/bin/env bash

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cilium_setup_paths() {
  cluster_setup_paths
  CILIUM_STATE_DIR="${STATE_DIR}/cilium"
}

cilium_parse_args() {
  CLUSTERS="${usage_clusters:-alpha,beta}"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --clusters)
        CLUSTERS="$2"
        shift 2
        ;;
      *)
        echo "Unknown argument: $1" >&2
        exit 1
        ;;
    esac
  done
}

cilium_cluster_list() {
  local item
  IFS=',' read -r -a items <<< "${CLUSTERS}"
  for item in "${items[@]}"; do
    item="$(echo "${item}" | xargs)"
    [[ -n "${item}" ]] && printf '%s\n' "${item}"
  done
}

cilium_kubeconfig_for() {
  local cluster="$1"
  local kubeconfig="${KUBECONFIG_DIR}/${cluster}.yaml"
  if [[ ! -f "${kubeconfig}" ]]; then
    echo "Missing kubeconfig for cluster '${cluster}': ${kubeconfig}" >&2
    exit 1
  fi
  printf '%s\n' "${kubeconfig}"
}

cilium_context_for() {
  local cluster="$1"
  cluster_context_for "${cluster}"
}

cilium_merged_kubeconfig() {
  local cluster
  local kubeconfig
  local merged=""

  for cluster in "$@"; do
    kubeconfig="$(cilium_kubeconfig_for "${cluster}")"
    if [[ -n "${merged}" ]]; then
      merged+="${PATH_SEPARATOR:-:}"
    fi
    merged+="${kubeconfig}"
  done

  printf '%s\n' "${merged}"
}

cilium_cli() {
  local kubeconfig_bundle="$1"
  shift
  KUBECONFIG="${kubeconfig_bundle}" cilium "$@"
}

cilium_cluster_id_for() {
  local cluster="$1"
  case "${cluster}" in
    alpha) printf '1\n' ;;
    beta) printf '2\n' ;;
    *)
      python3 - <<'PY' "${cluster}"
import sys

cluster = sys.argv[1]
print(10 + (sum(ord(c) for c in cluster) % 200))
PY
      ;;
  esac
}

cilium_require_base_tools() {
  require_cmd docker
  require_cmd kubectl
  require_cmd helm
  require_cmd cilium
  check_container_runtime
}
