#!/usr/bin/env bash
set -euo pipefail

# Lightweight single-cluster operator end-to-end test.
# Starts a k0s-in-Docker cluster, builds and loads the operator image,
# deploys the operator, and validates workload targeting, service routing,
# datastore injection, and degraded-to-ready recovery. No Cilium,
# ClusterMesh, or multi-cluster networking required.

REPO_ROOT="$(git rev-parse --show-toplevel)"

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_common.sh"
cluster_setup_paths

CLUSTER="e2e"
IMAGE="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:e2e}"
MANIFESTS_DIR="${REPO_ROOT}/examples/local-multicluster/manifests"
TIMEOUT="${E2E_TIMEOUT:-300}"
KUBECONFIG_PATH=""
CONTEXT=""

diagnostics() {
  [[ -n "${KUBECONFIG_PATH}" && -n "${CONTEXT}" ]] || return 0

  printf '==> e2e diagnostics\n' >&2
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get nodes -o wide >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" describe nodes >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get pods -A -o wide >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" describe pods -A >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get workloads -A -o yaml >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get serviceendpoints -A -o yaml >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get datastorebindings -A -o yaml >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get deployments,services -A -o yaml >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" get events -A --sort-by=.lastTimestamp >&2 || true
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" -n polykube-system logs deployment/polykube-operator --all-containers --tail=200 >&2 || true
}

kubectl_e2e() {
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" "$@"
}

wait_jsonpath_equals() {
  local expected="$1"
  local jsonpath="$2"
  shift 2
  local value=""

  for _ in $(seq 1 "${TIMEOUT}"); do
    value="$(kubectl_e2e "$@" -o "jsonpath=${jsonpath}" 2>/dev/null || true)"
    if [[ "${value}" == "${expected}" ]]; then
      return 0
    fi
    sleep 1
  done

  printf 'ERROR: expected %s to equal %s, got %s\n' "${jsonpath}" "${expected}" "${value}" >&2
  kubectl_e2e "$@" -o yaml >&2 || true
  return 1
}

assert_resource_absent() {
  if kubectl_e2e "$@" >/dev/null 2>&1; then
    printf 'ERROR: expected resource to be absent: kubectl %s\n' "$*" >&2
    kubectl_e2e "$@" -o yaml >&2 || true
    return 1
  fi
}

cleanup() {
  status=$?
  if [[ "${status}" -ne 0 ]]; then
    diagnostics
  fi
  printf '==> cleaning up cluster %s\n' "${CLUSTER}"
  bash "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_delete.sh" \
    --clusters "${CLUSTER}" 2>/dev/null || true
  exit "${status}"
}
trap cleanup EXIT

# 1. Cluster
printf '==> creating k0s cluster: %s\n' "${CLUSTER}"
bash "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_create.sh" \
  --clusters "${CLUSTER}" --workers 0 --network-provider kuberouter

KUBECONFIG_PATH="$(cluster_kubeconfig_for "${CLUSTER}")"
CONTEXT="$(cluster_context_for "${CLUSTER}")"

# 2. Build image while waiting for node to reach Ready in parallel
printf '==> building operator image: %s\n' "${IMAGE}"
kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  wait node --all --for=condition=Ready --timeout="${TIMEOUT}s" &
NODE_WAIT_PID=$!
docker build -t "${IMAGE}" -f "${REPO_ROOT}/operator/Dockerfile" "${REPO_ROOT}"

# 3. Load image into cluster
printf '==> loading image into cluster\n'
bash "${REPO_ROOT}/examples/local-multicluster/scripts/operator_image_load.sh" \
  --clusters "${CLUSTER}" --image "${IMAGE}"

# 4. Confirm node is Ready before deploying
printf '==> confirming node Ready\n'
wait "${NODE_WAIT_PID}"

# 5. Deploy CRDs, RBAC, and operator Deployment
printf '==> deploying operator to %s\n' "${CLUSTER}"
bash "${REPO_ROOT}/scripts/operator_deploy.sh" \
  --kubeconfig "${KUBECONFIG_PATH}" \
  --context "${CONTEXT}" \
  --image "${IMAGE}" \
  --cluster-member-name "${CLUSTER}"

# 6. Apply ClusterMember (inline), Federation, Workload, and ServiceEndpoint
printf '==> applying ClusterMember, Federation, Workload, ServiceEndpoint\n'
kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" apply -f - <<MANIFEST
apiVersion: infrastructure.polykube.dev/v1alpha1
kind: ClusterMember
metadata:
  name: ${CLUSTER}
  labels:
    env: dev
    provider: k0s
spec:
  provider: k0s
  region: local
  clusterName: ${CLUSTER}
MANIFEST

kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  apply -f "${MANIFESTS_DIR}/federation.yaml"

kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  apply -f "${MANIFESTS_DIR}/workload-echo.yaml"

kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  apply -f "${MANIFESTS_DIR}/serviceendpoint-echo.yaml"

# 7. Assert ClusterMember reaches Ready=True
printf '==> waiting for ClusterMember/%s Ready (timeout %ss)\n' "${CLUSTER}" "${TIMEOUT}"
kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  wait "clustermember/${CLUSTER}" \
  --for=condition=Ready \
  --timeout="${TIMEOUT}s"

# 8. Assert Workload has a reconciled target entry
printf '==> asserting Workload echo has a reconciled target\n'
state=""
for _ in $(seq 1 "${TIMEOUT}"); do
  state="$(kubectl_e2e get workload echo -n default -o jsonpath='{.status.targets[0].state}' 2>/dev/null || true)"
  if [[ "${state}" == "Reconciling" || "${state}" == "Available" ]]; then
    break
  fi
  sleep 1
done

if [[ "${state}" != "Reconciling" && "${state}" != "Available" ]]; then
  printf 'ERROR: workload target state is %q, want Reconciling or Available\n' "${state}" >&2
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
    get workload echo -n default -o yaml >&2
  exit 1
fi

printf '==> workload target state: %s\n' "${state}"

# 9. Verify active/active Cilium service annotations.
printf '==> asserting ServiceEndpoint annotations\n'
wait_jsonpath_equals "True" '{.status.conditions[?(@.type=="Ready")].status}' \
  -n default get serviceendpoint echo
wait_jsonpath_equals "true" '{.metadata.annotations.service\.cilium\.io/global}' \
  -n default get service echo
wait_jsonpath_equals "true" '{.metadata.annotations.service\.cilium\.io/shared}' \
  -n default get service echo

# 10. Verify a missing datastore secret degrades, then recovers and injects env vars.
printf '==> asserting DatastoreBinding missing-secret recovery\n'
kubectl_e2e apply -f - <<'MANIFEST'
apiVersion: data.polykube.dev/v1alpha1
kind: DatastoreBinding
metadata:
  name: primary
  namespace: default
spec:
  workloadRef:
    name: echo
  engine: postgres
  connectionRef:
    name: echo-database
  replicationMode: None
MANIFEST

wait_jsonpath_equals "True" '{.status.conditions[?(@.type=="Degraded")].status}' \
  -n default get datastorebinding primary
wait_jsonpath_equals "ConnectionSecretNotFound" '{.status.conditions[?(@.type=="Degraded")].reason}' \
  -n default get datastorebinding primary

kubectl_e2e apply -f - <<'MANIFEST'
apiVersion: v1
kind: Secret
metadata:
  name: echo-database
  namespace: default
type: Opaque
stringData:
  url: postgres://polykube@database.default.svc:5432/echo
MANIFEST

wait_jsonpath_equals "True" '{.status.conditions[?(@.type=="Ready")].status}' \
  -n default get datastorebinding primary
wait_jsonpath_equals "echo-database" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_URL")].valueFrom.secretKeyRef.name}' \
  -n default get deployment echo
wait_jsonpath_equals "url" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_URL")].valueFrom.secretKeyRef.key}' \
  -n default get deployment echo
wait_jsonpath_equals "None" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_REPLICATION_MODE")].value}' \
  -n default get deployment echo
wait_jsonpath_equals "echo-database" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.name}' \
  -n default get deployment echo
wait_jsonpath_equals "url" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.key}' \
  -n default get deployment echo

# 11. Verify exclusion leaves status pending and creates no runtime objects.
printf '==> asserting target-policy exclusion\n'
kubectl_e2e apply -f - <<'MANIFEST'
apiVersion: runtime.polykube.dev/v1alpha1
kind: Workload
metadata:
  name: excluded
  namespace: default
spec:
  federationRef:
    name: local-dev
  image: hashicorp/http-echo:latest
  replicas: 0
  ports:
    - name: http
      containerPort: 5678
  targetPolicy:
    members:
      - another-cluster
MANIFEST

wait_jsonpath_equals "Pending" '{.status.targets[0].state}' \
  -n default get workload excluded
wait_jsonpath_equals "ExcludedByTargetPolicy" '{.status.conditions[?(@.type=="Pending")].reason}' \
  -n default get workload excluded
assert_resource_absent -n default get deployment excluded
assert_resource_absent -n default get service excluded

printf '==> e2e passed\n'
