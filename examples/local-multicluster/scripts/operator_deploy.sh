#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd kubectl

clusters="alpha,beta"
image="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:dev}"
wait="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) clusters="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --wait) wait="$2"; shift 2 ;;
    *) shift ;;
  esac
done

IFS=',' read -r -a cluster_names <<<"${clusters}"
for cluster in "${cluster_names[@]}"; do
  kubeconfig="$(cluster_kubeconfig_for "${cluster}")"
  context="$(cluster_context_for "${cluster}")"
  printf 'deploying operator to %s\n' "${context}"
  "${REPO_ROOT}/scripts/operator_deploy.sh" --kubeconfig "${kubeconfig}" --context "${context}" --image "${image}" --wait "${wait}"
done
