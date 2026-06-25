#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd docker
check_container_runtime

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) usage_clusters="$2"; shift 2 ;;
    --all) usage_all="true"; shift ;;
    *) shift ;;
  esac
done

ALL="${usage_all:-false}"
CLUSTERS="${usage_clusters:-alpha,beta}"

delete_cluster() {
  local cluster="$1"
  local container
  while read -r container; do
    [[ -n "${container}" ]] || continue
    echo "Deleting container '${container}'..."
    docker rm -f "${container}" >/dev/null 2>&1 || true
    docker volume rm "${container}-varlib" >/dev/null 2>&1 || true
    docker volume rm "${container}-varlog" >/dev/null 2>&1 || true
  done < <(cluster_container_names_for "${cluster}")

  rm -f "$(cluster_kubeconfig_for "${cluster}")"
  rm -f "${CLUSTER_CONFIG_DIR}/${cluster}.yaml"
}

if [[ "${ALL}" == "true" ]]; then
  mapfile -t detected_clusters < <(docker ps -a --format '{{.Names}}' | sed -n 's/^polykube-\([^-][^-]*\)-.*/\1/p' | sort -u)
  if [[ "${#detected_clusters[@]}" -eq 0 ]]; then
    echo "No Polykube k0s clusters found."
    exit 0
  fi
  for cluster in "${detected_clusters[@]}"; do
    delete_cluster "${cluster}"
  done
else
  IFS=',' read -r -a cluster_list <<< "${CLUSTERS}"
  for cluster in "${cluster_list[@]}"; do
    cluster="$(echo "${cluster}" | xargs)"
    [[ -n "${cluster}" ]] || continue
    delete_cluster "${cluster}"
  done
fi

echo "Deleted requested clusters and kubeconfigs."
