#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_parse_args "$@"
cilium_require_base_tools

mkdir -p "${CILIUM_STATE_DIR}"

echo "## Tool Versions"
echo "kubectl=$(kubectl version --client=true -o json | python3 -c 'import json,sys; payload=json.load(sys.stdin); print(payload["clientVersion"]["gitVersion"])')"
echo "helm=$(helm version --template '{{.Version}}')"
echo "cilium=$(cilium version --client 2>/dev/null | sed -n 's/^cilium-cli: //p' | head -n 1)"
echo "docker=$(docker version --format '{{.Server.Version}}')"
echo "k0s-image=${K0S_IMAGE}"

echo
echo "## Cluster Prerequisites"
for cluster in $(cilium_cluster_list); do
  kubeconfig="$(cilium_kubeconfig_for "${cluster}")"
  context="$(cilium_context_for "${cluster}")"

  echo "cluster=${cluster} context=${context} kubeconfig=${kubeconfig}"
  kubectl --kubeconfig "${kubeconfig}" config get-contexts "${context}" >/dev/null
  kubectl --kubeconfig "${kubeconfig}" --context "${context}" get nodes -o wide
done

echo
echo "## Notes"
echo "- Existing k0s kubeconfigs/contexts are present for requested clusters."
echo "- Preflight passed."
