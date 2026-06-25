#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_require_base_tools

copy_cilium_ca_secret() {
  local source_kubeconfig="$1"
  local source_context="$2"
  local target_kubeconfig="$3"
  local target_context="$4"

  kubectl --kubeconfig "${source_kubeconfig}" --context "${source_context}" -n kube-system get secret cilium-ca -o json | \
    python3 -c 'import json,sys; payload=json.load(sys.stdin); metadata=payload.get("metadata", {}); labels=dict(metadata.get("labels") or {}); annotations=dict(metadata.get("annotations") or {}); labels["app.kubernetes.io/managed-by"]="Helm"; annotations["meta.helm.sh/release-name"]="cilium"; annotations["meta.helm.sh/release-namespace"]="kube-system"; json.dump({"apiVersion":"v1","kind":"Secret","metadata":{"name":metadata["name"],"namespace":metadata["namespace"],"labels":labels,"annotations":annotations},"type":payload.get("type","Opaque"),"data":payload.get("data",{})}, sys.stdout)' | kubectl --kubeconfig "${target_kubeconfig}" --context "${target_context}" apply -f - >/dev/null
}

CLUSTERS="${usage_clusters:-alpha,beta}"
VERSION="${usage_version:-1.18.2}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) CLUSTERS="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

ca_source_cluster=""
ca_source_context=""
ca_source_kubeconfig=""

for cluster in $(printf '%s' "${CLUSTERS}" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sed '/^$/d'); do
  context="$(cilium_context_for "${cluster}")"
  cluster_id="$(cilium_cluster_id_for "${cluster}")"
  kubeconfig="$(cilium_kubeconfig_for "${cluster}")"
  kubeconfig_bundle="$(cilium_merged_kubeconfig "${cluster}")"
  pod_cidr="$(cluster_pod_cidr_for "${cluster}")"
  node_count="$(kubectl --kubeconfig "${kubeconfig}" --context "${context}" get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"

  echo "## Install ${cluster}"
  if [[ "${node_count}" == "1" ]]; then
    echo "Single-node cluster detected; removing control-plane NoSchedule taints on ${context}"
    kubectl --kubeconfig "${kubeconfig}" --context "${context}" taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true
    kubectl --kubeconfig "${kubeconfig}" --context "${context}" taint nodes --all node-role.kubernetes.io/master- >/dev/null 2>&1 || true
  fi

  if kubectl --kubeconfig "${kubeconfig}" --context "${context}" -n kube-system get ds cilium >/dev/null 2>&1; then
    cilium_cli "${kubeconfig_bundle}" upgrade \
      --context "${context}" \
      --version "${VERSION}" \
      --set "cluster.name=${cluster}" \
      --set "cluster.id=${cluster_id}" \
      --set "ipam.operator.clusterPoolIPv4PodCIDRList={${pod_cidr}}" \
      --set "ipam.operator.clusterPoolIPv4MaskSize=24" \
      --wait
  else
    if [[ -n "${ca_source_cluster}" ]]; then
      echo "Reusing Cilium CA from ${ca_source_cluster} on ${cluster}"
      copy_cilium_ca_secret "${ca_source_kubeconfig}" "${ca_source_context}" "${kubeconfig}" "${context}"
    fi

    cilium_cli "${kubeconfig_bundle}" install \
      --context "${context}" \
      --version "${VERSION}" \
      --set "cluster.name=${cluster}" \
      --set "cluster.id=${cluster_id}" \
      --set "ipam.operator.clusterPoolIPv4PodCIDRList={${pod_cidr}}" \
      --set "ipam.operator.clusterPoolIPv4MaskSize=24" \
      --wait
  fi

  if [[ -z "${ca_source_cluster}" ]]; then
    ca_source_cluster="${cluster}"
    ca_source_context="${context}"
    ca_source_kubeconfig="${kubeconfig}"
  fi
done
