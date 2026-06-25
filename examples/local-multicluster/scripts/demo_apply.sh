#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd kubectl

clusters="alpha,beta"
manifests_dir="${REPO_ROOT}/examples/local-multicluster/manifests"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) clusters="$2"; shift 2 ;;
    *) shift ;;
  esac
done

IFS=',' read -r -a cluster_names <<<"${clusters}"
for cluster in "${cluster_names[@]}"; do
  kubeconfig="$(cluster_kubeconfig_for "${cluster}")"
  context="$(cluster_context_for "${cluster}")"
  printf 'applying demo manifests to %s\n' "${context}"
  for member in "${cluster_names[@]}"; do
    kubectl --kubeconfig "${kubeconfig}" --context "${context}" apply -f "${manifests_dir}/clustermember-${member}.yaml"
  done
  kubectl --kubeconfig "${kubeconfig}" --context "${context}" apply -f "${manifests_dir}/federation.yaml"
  kubectl --kubeconfig "${kubeconfig}" --context "${context}" apply -f "${manifests_dir}/workload-echo.yaml"
  kubectl --kubeconfig "${kubeconfig}" --context "${context}" apply -f "${manifests_dir}/serviceendpoint-echo.yaml"
done
