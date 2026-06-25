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

  echo "## Reset ${cluster}"
  if kubectl --kubeconfig "${kubeconfig}" --context "${context}" -n kube-system get ds cilium >/dev/null 2>&1; then
    cilium_cli "${kubeconfig_bundle}" uninstall --context "${context}" --wait || true
  else
    echo "Cilium not installed on ${context}; skipping uninstall"
  fi

  kubectl --kubeconfig "${kubeconfig}" --context "${context}" -n kube-system delete secret clustermesh-apiserver-ca clustermesh-apiserver-admin-cert clustermesh-apiserver-remote-cert cilium-ca --ignore-not-found >/dev/null 2>&1 || true
done

echo "Cilium reset complete"
