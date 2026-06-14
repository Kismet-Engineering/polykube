#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd docker
require_cmd kubectl
check_container_runtime

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) usage_clusters="$2"; shift 2 ;;
    --workers) usage_workers="$2"; shift 2 ;;
    --image) usage_image="$2"; shift 2 ;;
    --recreate) usage_recreate="$2"; shift 2 ;;
    *) shift ;;
  esac
done

CLUSTERS="${usage_clusters:-alpha,beta}"
WORKERS="${usage_workers:-0}"
K0S_IMAGE="${usage_image:-${K0S_IMAGE}}"
RECREATE="${usage_recreate:-false}"

if ! [[ "${WORKERS}" =~ ^[0-9]+$ ]]; then
  echo "Workers must be a non-negative integer (got: ${WORKERS})." >&2
  exit 1
fi

mkdir -p "${KUBECONFIG_DIR}" "${CLUSTER_LOG_DIR}" "${CLUSTER_CONFIG_DIR}"

docker_run_k0s() {
  local name="$1"
  shift
  docker run -d --name "${name}" --hostname "${name}" \
    --privileged \
    --ulimit nofile=65536:65536 \
    --tmpfs /run \
    --tmpfs /tmp \
    -v "/dev/kmsg:/dev/kmsg:ro" \
    -v "${name}-varlib:/var/lib/k0s" \
    -v "${name}-varlog:/var/log/pods" \
    "$@"
}

configure_container_mounts() {
  local name="$1"
  docker exec "${name}" /bin/sh -lc 'mkdir -p /run/cilium/cgroupv2 /var/run/netns && mount --make-rshared / && if ! mountpoint -q /var/run/netns; then mount --bind /var/run/netns /var/run/netns; fi && mount --make-rshared /var/run/netns && mount --make-rshared /sys && mount --make-rshared /run' >/dev/null 2>&1 || {
    echo "Failed to enable shared mount propagation in ${name}." >&2
    return 1
  }
}

cluster_delete_runtime() {
  local cluster="$1"
  local container
  while read -r container; do
    [[ -n "${container}" ]] || continue
    docker rm -f "${container}" >/dev/null 2>&1 || true
    docker volume rm "${container}-varlib" >/dev/null 2>&1 || true
    docker volume rm "${container}-varlog" >/dev/null 2>&1 || true
  done < <(cluster_container_names_for "${cluster}")
}

wait_for_controller() {
  local cluster="$1"
  local controller_name
  controller_name="$(cluster_controller_name "${cluster}")"
  for _ in $(seq 1 60); do
    if docker exec "${controller_name}" k0s kubectl get ns >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

wait_for_worker_token() {
  local cluster="$1"
  local controller_name
  controller_name="$(cluster_controller_name "${cluster}")"
  local token=""

  for _ in $(seq 1 60); do
    token="$(docker exec "${controller_name}" k0s token create --role=worker 2>/dev/null || true)"
    if [[ -n "${token}" ]]; then
      printf '%s\n' "${token}"
      return 0
    fi
    sleep 2
  done

  return 1
}

wait_for_nodes() {
  local cluster="$1"
  local expected_nodes="$2"
  local kubeconfig
  kubeconfig="$(cluster_kubeconfig_for "${cluster}")"
  local context
  context="$(cluster_context_for "${cluster}")"
  local controller_name
  controller_name="$(cluster_controller_name "${cluster}")"
  local count="0"
  for _ in $(seq 1 120); do
    count="$(kubectl --kubeconfig "${kubeconfig}" --context "${context}" get nodes -o name 2>/dev/null | wc -l | tr -d ' ')"
    if [[ "${count}" == "0" ]]; then
      count="$(docker exec "${controller_name}" k0s kubectl get nodes -o name 2>/dev/null | wc -l | tr -d ' ')"
    fi
    if [[ "${count}" == "${expected_nodes}" ]]; then
      return 0
    fi
    sleep 2
  done
  echo "Cluster '${cluster}' reached ${count} node(s); expected ${expected_nodes}." >&2
  return 1
}

write_kubeconfig() {
  local cluster="$1"
  local controller_name
  controller_name="$(cluster_controller_name "${cluster}")"
  local api_port
  api_port="$(cluster_api_port_for "${cluster}")"
  local kubeconfig
  kubeconfig="$(cluster_kubeconfig_for "${cluster}")"
  docker exec "${controller_name}" k0s kubeconfig admin >"${kubeconfig}"

  python3 - <<'PY' "${kubeconfig}" "${cluster}" "${api_port}" "${CLUSTER_CONTEXT_PREFIX}"
from pathlib import Path
import re
import sys

kubeconfig_path, cluster, api_port, prefix = sys.argv[1:]
context_name = f"{prefix}-{cluster}"
cluster_name = context_name
user_name = f"admin-{cluster}"

lines = Path(kubeconfig_path).read_text(encoding="utf-8").splitlines()
section = None
list_item = None

for idx, line in enumerate(lines):
    stripped = line.strip()

    if stripped in {"clusters:", "contexts:", "users:"}:
        section = stripped[:-1]
        list_item = None
        continue

    if stripped.startswith("server: https://"):
        lines[idx] = re.sub(r"server: https://[^\n]+:6443", f"server: https://127.0.0.1:{api_port}", line)
        continue

    if stripped.startswith("current-context:"):
        lines[idx] = f"current-context: {context_name}"
        continue

    if stripped.startswith("- cluster:"):
        list_item = "cluster"
        continue

    if stripped.startswith("- context:"):
        list_item = "context"
        continue

    if stripped.startswith("- name:"):
        if section == "clusters":
            lines[idx] = f"- name: {cluster_name}"
            list_item = "cluster"
        elif section == "contexts":
            lines[idx] = f"- name: {context_name}"
            list_item = "context"
        elif section == "users":
            lines[idx] = f"- name: {user_name}"
            list_item = "user"
        continue

    if stripped.startswith("name:"):
        if section == "clusters" and list_item == "cluster":
            lines[idx] = "  name: " + cluster_name
        elif section == "contexts" and list_item == "context":
            lines[idx] = "  name: " + context_name
        continue

    if section == "contexts" and list_item == "context":
        if stripped.startswith("cluster:"):
            lines[idx] = "    cluster: " + cluster_name
        elif stripped.startswith("user:"):
            lines[idx] = "    user: " + user_name

Path(kubeconfig_path).write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
}

create_cluster() {
  local cluster="$1"
  local workers="$2"
  local controller_name
  controller_name="$(cluster_controller_name "${cluster}")"
  local config_path="${CLUSTER_CONFIG_DIR}/${cluster}.yaml"
  local api_port
  api_port="$(cluster_api_port_for "${cluster}")"

  cluster_write_config "${cluster}" "${config_path}"

  echo "Creating k0s cluster '${cluster}' with ${workers} extra worker(s)..."
  docker_run_k0s "${controller_name}" \
    -p "${api_port}:6443" \
    -v "${config_path}:/etc/k0s/k0s.yaml:ro" \
    "${K0S_IMAGE}" \
    k0s controller --enable-worker --config /etc/k0s/k0s.yaml >/dev/null

  configure_container_mounts "${controller_name}"

  if ! wait_for_controller "${cluster}"; then
    cluster_capture_failure_logs "${cluster}"
    echo "k0s controller for cluster '${cluster}' failed to become ready." >&2
    exit 1
  fi

  if [[ "${workers}" -gt 0 ]]; then
    local token
    if ! token="$(wait_for_worker_token "${cluster}")"; then
      cluster_capture_failure_logs "${cluster}"
      echo "k0s controller for cluster '${cluster}' did not become ready to issue worker tokens." >&2
      exit 1
    fi
    local worker_index
    for worker_index in $(seq 1 "${workers}"); do
      local worker_name
      worker_name="$(cluster_worker_name "${cluster}" "${worker_index}")"
      docker_run_k0s "${worker_name}" "${K0S_IMAGE}" k0s worker "${token}" >/dev/null
      configure_container_mounts "${worker_name}"
    done
  fi

  write_kubeconfig "${cluster}"
  if ! wait_for_nodes "${cluster}" "$((workers + 1))"; then
    cluster_capture_failure_logs "${cluster}"
    echo "Cluster '${cluster}' did not finish bootstrapping." >&2
    echo "Inspect ${CLUSTER_LOG_DIR}/${cluster} for diagnostics." >&2
    exit 1
  fi
}

IFS=',' read -r -a CLUSTER_LIST <<< "${CLUSTERS}"
for cluster in "${CLUSTER_LIST[@]}"; do
  cluster="$(echo "${cluster}" | xargs)"
  [[ -n "${cluster}" ]] || continue

  if cluster_exists "${cluster}"; then
    if [[ "${RECREATE}" == "true" ]]; then
      echo "Deleting existing cluster '${cluster}'..."
      cluster_delete_runtime "${cluster}"
    else
      echo "Cluster '${cluster}' already exists. Reusing."
      write_kubeconfig "${cluster}"
      continue
    fi
  fi

  create_cluster "${cluster}" "${WORKERS}"
done

echo "Kubeconfigs written to ${KUBECONFIG_DIR}"
echo "To use all clusters:"
echo "  export KUBECONFIG=$(ls -1 "${KUBECONFIG_DIR}"/*.yaml | paste -sd: -)"
