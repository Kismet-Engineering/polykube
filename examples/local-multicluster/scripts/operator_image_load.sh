#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd docker
check_container_runtime

clusters="alpha,beta"
image="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:dev}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) clusters="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    *) shift ;;
  esac
done

IFS=',' read -r -a cluster_names <<<"${clusters}"
for cluster in "${cluster_names[@]}"; do
  while read -r container; do
    [[ -n "${container}" ]] || continue
    printf 'loading %s into %s\n' "${image}" "${container}"
    docker save "${image}" | docker exec -i "${container}" k0s ctr -n k8s.io images import - >/dev/null
  done < <(cluster_container_names_for "${cluster}")
done
