#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd kubectl

clusters="alpha,beta"
delete_crds="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) clusters="$2"; shift 2 ;;
    --crds) delete_crds="$2"; shift 2 ;;
    *) shift ;;
  esac
done

IFS=',' read -r -a cluster_names <<<"${clusters}"
for cluster in "${cluster_names[@]}"; do
  kubeconfig="$(cluster_kubeconfig_for "${cluster}")"
  context="$(cluster_context_for "${cluster}")"
  printf 'removing operator from %s\n' "${context}"
  "${REPO_ROOT}/scripts/operator_undeploy.sh" --kubeconfig "${kubeconfig}" --context "${context}" --crds "${delete_crds}"
done
