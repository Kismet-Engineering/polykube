#!/usr/bin/env bash
set -euo pipefail

# Lightweight single-cluster operator smoke test.
# Starts a k0s-in-Docker cluster, builds and loads the operator image,
# deploys the operator, applies ClusterMember/Federation/Workload, and
# asserts that reconciliation completes. No Cilium, ClusterMesh, or
# multi-cluster networking required.

REPO_ROOT="$(git rev-parse --show-toplevel)"

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_common.sh"
cluster_setup_paths

CLUSTER="e2e"
IMAGE="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:e2e}"
MANIFESTS_DIR="${REPO_ROOT}/examples/local-multicluster/manifests"
TIMEOUT="${E2E_TIMEOUT:-300}"

cleanup() {
  printf '==> cleaning up cluster %s\n' "${CLUSTER}"
  bash "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_delete.sh" \
    --clusters "${CLUSTER}" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Cluster
printf '==> creating k0s cluster: %s\n' "${CLUSTER}"
bash "${REPO_ROOT}/examples/local-multicluster/scripts/cluster_create.sh" \
  --clusters "${CLUSTER}" --workers 0

KUBECONFIG_PATH="$(cluster_kubeconfig_for "${CLUSTER}")"
CONTEXT="$(cluster_context_for "${CLUSTER}")"

# 2. Image
printf '==> building operator image: %s\n' "${IMAGE}"
docker build -t "${IMAGE}" -f "${REPO_ROOT}/operator/Dockerfile" "${REPO_ROOT}"

# 3. Load image into cluster
printf '==> loading image into cluster\n'
bash "${REPO_ROOT}/examples/local-multicluster/scripts/operator_image_load.sh" \
  --clusters "${CLUSTER}" --image "${IMAGE}"

# 4. Deploy CRDs, RBAC, and operator Deployment
printf '==> deploying operator to %s\n' "${CLUSTER}"
bash "${REPO_ROOT}/scripts/operator_deploy.sh" \
  --kubeconfig "${KUBECONFIG_PATH}" \
  --context "${CONTEXT}" \
  --image "${IMAGE}" \
  --cluster-member-name "${CLUSTER}"

# 5. Apply ClusterMember (inline), Federation, and Workload
printf '==> applying ClusterMember, Federation, Workload\n'
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

# 6. Assert ClusterMember reaches Ready=True
printf '==> waiting for ClusterMember/%s Ready (timeout %ss)\n' "${CLUSTER}" "${TIMEOUT}"
kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  wait "clustermember/${CLUSTER}" \
  --for=condition=Ready \
  --timeout="${TIMEOUT}s"

# 7. Assert Workload has a reconciled target entry
printf '==> asserting Workload echo has a reconciled target\n'
state="$(kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
  get workload echo -n default \
  -o jsonpath='{.status.targets[0].state}')"

if [[ "${state}" != "Reconciling" && "${state}" != "Available" ]]; then
  printf 'ERROR: workload target state is %q, want Reconciling or Available\n' "${state}" >&2
  kubectl --kubeconfig "${KUBECONFIG_PATH}" --context "${CONTEXT}" \
    get workload echo -n default -o yaml >&2
  exit 1
fi

printf '==> workload target state: %s\n' "${state}"
printf '==> e2e passed\n'
