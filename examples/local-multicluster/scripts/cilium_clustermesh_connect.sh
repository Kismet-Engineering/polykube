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

echo "## Connect ${SOURCE} -> ${DESTINATION}"
cilium_cli "${KUBECONFIG_BUNDLE}" clustermesh connect --context "${SOURCE_CONTEXT}" --destination-context "${DEST_CONTEXT}"
