#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_require_base_tools

SOURCE="${usage_source:-alpha}"
DESTINATION="${usage_destination:-beta}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source) SOURCE="$2"; shift 2 ;;
    --destination) DESTINATION="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

SOURCE_CONTEXT="$(cilium_context_for "${SOURCE}")"
DEST_CONTEXT="$(cilium_context_for "${DESTINATION}")"
KUBECONFIG_BUNDLE="$(cilium_merged_kubeconfig "${SOURCE}" "${DESTINATION}")"
RUN_CONNECTIVITY_TEST="${CILIUM_VERIFY_RUN_CONNECTIVITY_TEST:-false}"
STATUS_WAIT_DURATION="${CILIUM_VERIFY_STATUS_WAIT_DURATION:-45s}"

echo "## Cilium Status"
cilium_cli "${KUBECONFIG_BUNDLE}" status --context "${SOURCE_CONTEXT}" --wait --wait-duration "${STATUS_WAIT_DURATION}"
cilium_cli "${KUBECONFIG_BUNDLE}" status --context "${DEST_CONTEXT}" --wait --wait-duration "${STATUS_WAIT_DURATION}"

echo
echo "## ClusterMesh Status"
cilium_cli "${KUBECONFIG_BUNDLE}" clustermesh status --context "${SOURCE_CONTEXT}" --wait
cilium_cli "${KUBECONFIG_BUNDLE}" clustermesh status --context "${DEST_CONTEXT}" --wait

echo
echo "## Connectivity Test"
if [[ "${RUN_CONNECTIVITY_TEST}" == "true" ]]; then
  cilium_cli "${KUBECONFIG_BUNDLE}" connectivity test --context "${SOURCE_CONTEXT}" --multi-cluster "${DEST_CONTEXT}"
else
  echo "Skipping cilium connectivity test by default; set CILIUM_VERIFY_RUN_CONNECTIVITY_TEST=true to enable it."
fi
