#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cilium_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cilium_common.sh"

cilium_setup_paths
cilium_require_base_tools

SOURCE="${usage_source:-alpha}"
DESTINATION="${usage_destination:-beta}"
NAMESPACE="${usage_namespace:-cilium-mesh-test}"
SERVICE_NAME="${usage_service:-mesh-echo}"
ROLLOUT_TIMEOUT="${CILIUM_GLOBAL_SERVICE_ROLLOUT_TIMEOUT:-60s}"
CLIENT_READY_TIMEOUT="${CILIUM_GLOBAL_SERVICE_CLIENT_READY_TIMEOUT:-45s}"
PROBE_ATTEMPTS="${CILIUM_GLOBAL_SERVICE_PROBE_ATTEMPTS:-10}"
PROBE_INTERVAL_SECONDS="${CILIUM_GLOBAL_SERVICE_PROBE_INTERVAL_SECONDS:-2}"
PROBE_SAMPLES_PER_ATTEMPT="${CILIUM_GLOBAL_SERVICE_PROBE_SAMPLES_PER_ATTEMPT:-12}"
PROBE_MAX_TIME_SECONDS="${CILIUM_GLOBAL_SERVICE_PROBE_MAX_TIME_SECONDS:-3}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source) SOURCE="$2"; shift 2 ;;
    --destination) DESTINATION="$2"; shift 2 ;;
    --namespace) NAMESPACE="$2"; shift 2 ;;
    --service) SERVICE_NAME="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

SOURCE_CONTEXT="$(cilium_context_for "${SOURCE}")"
DEST_CONTEXT="$(cilium_context_for "${DESTINATION}")"
SOURCE_KUBECONFIG="$(cilium_kubeconfig_for "${SOURCE}")"
DEST_KUBECONFIG="$(cilium_kubeconfig_for "${DESTINATION}")"

apply_cluster_workload() {
  local kubeconfig="$1"
  local context="$2"
  local cluster_name="$3"

  cilium_kubectl "${kubeconfig}" --context "${context}" create namespace "${NAMESPACE}" --dry-run=client -o yaml | cilium_kubectl "${kubeconfig}" --context "${context}" apply -f - >/dev/null
  cilium_kubectl "${kubeconfig}" --context "${context}" apply -f - <<EOF >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${SERVICE_NAME}
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${SERVICE_NAME}
  template:
    metadata:
      labels:
        app: ${SERVICE_NAME}
    spec:
      containers:
        - name: http
          image: busybox:1.36
          command: ["sh", "-lc", "mkdir -p /www && echo ${cluster_name} > /www/index.html && exec httpd -f -p 8080 -h /www"]
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: ${SERVICE_NAME}
  namespace: ${NAMESPACE}
  annotations:
    service.cilium.io/global: "true"
spec:
  selector:
    app: ${SERVICE_NAME}
  ports:
    - port: 80
      targetPort: 8080
EOF
}

echo "## Apply Global Service"
apply_cluster_workload "${SOURCE_KUBECONFIG}" "${SOURCE_CONTEXT}" "${SOURCE}"
apply_cluster_workload "${DEST_KUBECONFIG}" "${DEST_CONTEXT}" "${DESTINATION}"

cilium_kubectl "${SOURCE_KUBECONFIG}" --context "${SOURCE_CONTEXT}" -n "${NAMESPACE}" rollout status "deployment/${SERVICE_NAME}" --timeout="${ROLLOUT_TIMEOUT}" >/dev/null
cilium_kubectl "${DEST_KUBECONFIG}" --context "${DEST_CONTEXT}" -n "${NAMESPACE}" rollout status "deployment/${SERVICE_NAME}" --timeout="${ROLLOUT_TIMEOUT}" >/dev/null

cilium_kubectl "${SOURCE_KUBECONFIG}" --context "${SOURCE_CONTEXT}" -n "${NAMESPACE}" get pod "${SERVICE_NAME}-client" >/dev/null 2>&1 || cilium_kubectl "${SOURCE_KUBECONFIG}" --context "${SOURCE_CONTEXT}" -n "${NAMESPACE}" run "${SERVICE_NAME}-client" --image=curlimages/curl:8.12.1 --command -- sleep 3600 >/dev/null
cilium_kubectl "${SOURCE_KUBECONFIG}" --context "${SOURCE_CONTEXT}" -n "${NAMESPACE}" wait --for=condition=Ready "pod/${SERVICE_NAME}-client" --timeout="${CLIENT_READY_TIMEOUT}" >/dev/null

echo "## Probe Responses"
probe_host="http://${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"

collect_responses() {
  cilium_kubectl "${SOURCE_KUBECONFIG}" --context "${SOURCE_CONTEXT}" -n "${NAMESPACE}" exec "${SERVICE_NAME}-client" -- \
    sh -lc "set -e; i=1; while [ \$i -le ${PROBE_SAMPLES_PER_ATTEMPT} ]; do curl -fsS --max-time ${PROBE_MAX_TIME_SECONDS} ${probe_host} || true; echo; i=\$((i+1)); done" | tr -d '\r'
}

responses=""
for _ in $(seq 1 "${PROBE_ATTEMPTS}"); do
  responses="$(collect_responses)"
  if [[ "${responses}" == *"${SOURCE}"* && "${responses}" == *"${DESTINATION}"* ]]; then
    break
  fi
  sleep "${PROBE_INTERVAL_SECONDS}"
done

printf '%s\n' "${responses}" | sed '/^$/d' | sort | uniq | while read -r line; do
  echo "RESPONSE|${line}"
done

if [[ "${responses}" != *"${SOURCE}"* ]]; then
  echo "Did not observe local cluster response '${SOURCE}'" >&2
  exit 1
fi

if [[ "${responses}" != *"${DESTINATION}"* ]]; then
  echo "Did not observe remote cluster response '${DESTINATION}'" >&2
  exit 1
fi

echo "SUMMARY status=pass source=${SOURCE} destination=${DESTINATION} service=${SERVICE_NAME} namespace=${NAMESPACE}"
