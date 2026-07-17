#!/usr/bin/env bash

cluster_setup_paths() {
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  EXAMPLE_DIR="$(cd "${script_dir}/.." && pwd)"
  REPO_ROOT="$(cd "${EXAMPLE_DIR}/../.." && pwd)"
  STATE_DIR="${EXAMPLE_DIR}/state"
  KUBECONFIG_DIR="${STATE_DIR}/kubeconfigs"
  CLUSTER_LOG_DIR="${STATE_DIR}/cluster-logs"
  CLUSTER_CONFIG_DIR="${STATE_DIR}/cluster-configs"
  CLUSTER_CONTEXT_PREFIX="${CLUSTER_CONTEXT_PREFIX:-polykube}"
  K0S_IMAGE="${K0S_IMAGE:-docker.io/k0sproject/k0s:v1.35.2-k0s.0}"
  K0S_NETWORK_PROVIDER="${K0S_NETWORK_PROVIDER:-custom}"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    if [[ "$1" == "docker" ]]; then
      echo "Missing required command: docker" >&2
      echo "Install Docker CLI and start a Docker-compatible runtime." >&2
      exit 1
    fi
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

check_container_runtime() {
  require_cmd docker

  if [[ "${CI:-}" == "true" ]]; then
    return 0
  fi

  if command -v colima >/dev/null 2>&1; then
    local status_output
    status_output="$(colima status 2>&1 || true)"
    if ! printf '%s' "${status_output}" | grep -qi "colima is running"; then
      echo "Colima is installed but not running. Start it with: colima start" >&2
      exit 1
    fi

    local inotify_instances
    inotify_instances="$(colima ssh -- cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || true)"
    if [[ "${inotify_instances}" =~ ^[0-9]+$ && "${inotify_instances}" -lt 512 ]]; then
      echo "Colima fs.inotify.max_user_instances=${inotify_instances}; Polykube local clusters require at least 512." >&2
      echo "Raise it for the current VM with:" >&2
      echo "  colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=512" >&2
      echo "Persist it with a system provision script in ~/.colima/default/colima.yaml; see examples/local-multicluster/README.md." >&2
      exit 1
    fi
  fi
}

cluster_context_for() {
  local cluster="$1"
  printf '%s-%s\n' "${CLUSTER_CONTEXT_PREFIX}" "${cluster}"
}

cluster_kubeconfig_for() {
  local cluster="$1"
  printf '%s/%s.yaml\n' "${KUBECONFIG_DIR}" "${cluster}"
}

cluster_controller_name() {
  local cluster="$1"
  printf 'polykube-%s-controller\n' "${cluster}"
}

cluster_worker_name() {
  local cluster="$1"
  local worker_index="$2"
  printf 'polykube-%s-worker-%s\n' "${cluster}" "${worker_index}"
}

cluster_api_port_for() {
  local cluster="$1"
  case "${cluster}" in
    alpha) printf '16443\n' ;;
    beta) printf '17443\n' ;;
    *)
      python3 - <<'PY' "${cluster}"
import sys

cluster = sys.argv[1]
base = 18000
offset = sum(ord(c) for c in cluster) % 1000
print(base + offset)
PY
      ;;
  esac
}

cluster_index_for() {
  local cluster="$1"
  case "${cluster}" in
    alpha) printf '44\n' ;;
    beta) printf '45\n' ;;
    *)
      python3 - <<'PY' "${cluster}"
import sys

cluster = sys.argv[1]
print(50 + (sum(ord(c) for c in cluster) % 100))
PY
      ;;
  esac
}

cluster_pod_cidr_for() {
  local cluster="$1"
  local index
  index="$(cluster_index_for "${cluster}")"
  printf '10.%s.0.0/16\n' "${index}"
}

cluster_service_cidr_for() {
  local cluster="$1"
  local index
  index="$(cluster_index_for "${cluster}")"
  printf '10.%s.0.0/16\n' "$((index + 100))"
}

cluster_container_names_for() {
  local cluster="$1"
  docker ps -a --format '{{.Names}}' | grep -E "^polykube-${cluster}-(controller|worker-[0-9]+)$" || true
}

cluster_exists() {
  local cluster="$1"
  [[ -n "$(cluster_container_names_for "${cluster}")" ]]
}

cluster_write_config() {
  local cluster="$1"
  local output_path="$2"
  cat >"${output_path}" <<EOF
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: polykube-${cluster}
spec:
  api:
    sans:
      - 127.0.0.1
      - localhost
  network:
    provider: ${K0S_NETWORK_PROVIDER}
    podCIDR: $(cluster_pod_cidr_for "${cluster}")
    serviceCIDR: $(cluster_service_cidr_for "${cluster}")
  telemetry:
    enabled: false
EOF
}

cluster_capture_failure_logs() {
  local cluster="$1"
  local output_dir="${CLUSTER_LOG_DIR}/${cluster}"
  mkdir -p "${output_dir}"

  local container
  while read -r container; do
    [[ -n "${container}" ]] || continue
    docker logs "${container}" >"${output_dir}/${container}.docker.log" 2>&1 || true
    docker inspect "${container}" >"${output_dir}/${container}.inspect.json" 2>/dev/null || true
    docker exec "${container}" journalctl -u k0scontroller -n 200 --no-pager >"${output_dir}/${container}.k0scontroller.log" 2>&1 || true
    docker exec "${container}" journalctl -u k0sworker -n 200 --no-pager >"${output_dir}/${container}.k0sworker.log" 2>&1 || true
    docker exec "${container}" k0s kubectl get nodes -o wide >"${output_dir}/${container}.nodes.log" 2>&1 || true
  done < <(cluster_container_names_for "${cluster}")

  echo "Captured cluster diagnostics under ${output_dir}" >&2
}
