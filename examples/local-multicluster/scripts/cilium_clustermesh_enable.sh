#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_require_base_tools

CLUSTERS="${usage_clusters:-alpha,beta}"
SERVICE_TYPE="${usage_service_type:-NodePort}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) CLUSTERS="$2"; shift 2 ;;
    --service-type) SERVICE_TYPE="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

for cluster in $(printf '%s' "${CLUSTERS}" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sed '/^$/d'); do
  context="$(cilium_context_for "${cluster}")"
  kubeconfig_bundle="$(cilium_merged_kubeconfig "${cluster}")"
  echo "## Enable ClusterMesh on ${cluster}"
  cilium_cli "${kubeconfig_bundle}" clustermesh enable --context "${context}" --service-type "${SERVICE_TYPE}"
done
