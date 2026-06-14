#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_parse_args "$@"
cilium_require_base_tools

for cluster in $(cilium_cluster_list); do
  kubeconfig="$(cilium_kubeconfig_for "${cluster}")"
  context="$(cilium_context_for "${cluster}")"
  kubeconfig_bundle="$(cilium_merged_kubeconfig "${cluster}")"

  echo "## ${cluster}"
  if kubectl --kubeconfig "${kubeconfig}" --context "${context}" -n kube-system get ds cilium >/dev/null 2>&1; then
    cilium_cli "${kubeconfig_bundle}" status --context "${context}" --wait --wait-duration 180s || true
    if kubectl --kubeconfig "${kubeconfig}" --context "${context}" -n kube-system get svc clustermesh-apiserver >/dev/null 2>&1; then
      cilium_cli "${kubeconfig_bundle}" clustermesh status --context "${context}" --wait || true
    else
      echo "ClusterMesh not enabled on ${context}"
    fi
  else
    echo "Cilium not installed on ${context}"
  fi
  echo
done
