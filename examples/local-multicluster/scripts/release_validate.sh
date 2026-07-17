#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=examples/local-multicluster/scripts/cluster_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/cluster_common.sh"

cluster_setup_paths
require_cmd kubectl
require_cmd mise

clusters="alpha,beta"
workers="0"
image="polykube-operator:dev"
recreate="false"
evidence_dir="${STATE_DIR}/release-evidence"
wait_attempts="${POLYKUBE_RELEASE_VALIDATE_WAIT_ATTEMPTS:-180}"
wait_interval_seconds="${POLYKUBE_RELEASE_VALIDATE_WAIT_INTERVAL_SECONDS:-2}"

usage() {
  cat <<'EOF'
Usage: release_validate.sh [flags]

Flags:
  --clusters <clusters>       Comma-separated cluster names (default: alpha,beta)
  --workers <count>           Extra workers per cluster (default: 0)
  --image <image>             Operator image reference (default: polykube-operator:dev)
  --recreate <bool>           Recreate clusters if they already exist (default: false)
  --evidence-dir <path>       Directory for evidence logs (default: local state/release-evidence)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clusters) clusters="$2"; shift 2 ;;
    --workers) workers="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --recreate) recreate="$2"; shift 2 ;;
    --evidence-dir) evidence_dir="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

IFS=',' read -r -a cluster_names <<<"${clusters}"
if [[ "${#cluster_names[@]}" -ne 2 ]]; then
  echo "Release validation currently requires exactly two clusters." >&2
  exit 1
fi

source_cluster="${cluster_names[0]}"
destination_cluster="${cluster_names[1]}"
timestamp="$(date +%Y%m%d-%H%M%S)"
mkdir -p "${evidence_dir}"
evidence_log="${evidence_dir}/local-release-validation-${timestamp}.log"
exec > >(tee "${evidence_log}") 2>&1

printf 'Release validation evidence: %s\n' "${evidence_log}"
printf 'Clusters: %s\n' "${clusters}"
printf 'Operator image: %s\n' "${image}"

run_step() {
  printf '\n## %s\n' "$1"
  shift
  printf 'COMMAND:'
  printf ' %q' "$@"
  printf '\n'
  "$@"
}

kubectl_cluster() {
  local cluster="$1"
  shift
  kubectl --kubeconfig "$(cluster_kubeconfig_for "${cluster}")" --context "$(cluster_context_for "${cluster}")" "$@"
}

diagnostics() {
  printf '\n## Failure Diagnostics\n' >&2
  for cluster in "${cluster_names[@]}"; do
    printf '\n### Cluster %s\n' "${cluster}" >&2
    kubectl_cluster "${cluster}" get nodes -o wide >&2 || true
    kubectl_cluster "${cluster}" get pods -A -o wide >&2 || true
    kubectl_cluster "${cluster}" describe pods -A >&2 || true
    kubectl_cluster "${cluster}" get workloads -A -o yaml >&2 || true
    kubectl_cluster "${cluster}" get serviceendpoints -A -o yaml >&2 || true
    kubectl_cluster "${cluster}" get datastorebindings -A -o yaml >&2 || true
    kubectl_cluster "${cluster}" get deployments,services -A -o yaml >&2 || true
    kubectl_cluster "${cluster}" get events -A --sort-by=.lastTimestamp >&2 || true
    kubectl_cluster "${cluster}" -n polykube-system logs deployment/polykube-operator --all-containers --tail=200 >&2 || true
  done
}

on_exit() {
  local status=$?
  if [[ "${status}" -ne 0 ]]; then
    diagnostics
  fi
}
trap on_exit EXIT

wait_jsonpath_equals() {
  local cluster="$1"
  local expected="$2"
  local jsonpath="$3"
  shift 3
  local value=""

  for _ in $(seq 1 "${wait_attempts}"); do
    value="$(kubectl_cluster "${cluster}" "$@" -o "jsonpath=${jsonpath}" 2>/dev/null || true)"
    if [[ "${value}" == "${expected}" ]]; then
      return 0
    fi
    sleep "${wait_interval_seconds}"
  done

  printf 'Expected %s to equal %s, got %s\n' "${jsonpath}" "${expected}" "${value}" >&2
  kubectl_cluster "${cluster}" "$@" -o yaml >&2 || true
  return 1
}

wait_resource_exists() {
  local cluster="$1"
  shift
  for _ in $(seq 1 "${wait_attempts}"); do
    if kubectl_cluster "${cluster}" "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${wait_interval_seconds}"
  done

  printf 'Timed out waiting for resource in cluster %s: kubectl %s\n' "${cluster}" "$*" >&2
  return 1
}

assert_resource_absent() {
  local cluster="$1"
  shift
  if kubectl_cluster "${cluster}" "$@" >/dev/null 2>&1; then
    printf 'Expected resource to be absent in cluster %s: kubectl %s\n' "${cluster}" "$*" >&2
    kubectl_cluster "${cluster}" "$@" -o yaml >&2 || true
    return 1
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    printf '%s\nExpected to find: %s\nActual: %s\n' "${message}" "${needle}" "${haystack}" >&2
    return 1
  fi
}

kubeconfig_bundle=""
for cluster in "${cluster_names[@]}"; do
  kubeconfig_bundle+="$(cluster_kubeconfig_for "${cluster}"):"
done
export KUBECONFIG="${kubeconfig_bundle%:}"

run_step "Repository Validation" bash "${REPO_ROOT}/scripts/validate-repo.sh"
run_step "Create Local Clusters" mise run local:cluster:create -- --clusters "${clusters}" --workers "${workers}" --recreate "${recreate}"
run_step "Cluster Status" mise run local:cluster:status
run_step "Kubernetes Contexts" kubectl config get-contexts

run_step "Cilium Preflight" mise run local:cilium:preflight -- --clusters "${clusters}"
run_step "Cilium Install" mise run local:cilium:install -- --clusters "${clusters}"
run_step "Enable ClusterMesh" mise run local:cilium:clustermesh:enable -- --clusters "${clusters}" --service-type NodePort
run_step "Connect ClusterMesh" mise run local:cilium:clustermesh:connect -- --source "${source_cluster}" --destination "${destination_cluster}"
run_step "Verify Cilium And ClusterMesh" mise run local:cilium:verify -- --source "${source_cluster}" --destination "${destination_cluster}"
run_step "Probe Cilium Global Service" mise run local:cilium:global-service:probe -- --source "${source_cluster}" --destination "${destination_cluster}"

run_step "Build Operator Image" mise run operator:image:build -- --image "${image}"
run_step "Load Operator Image" mise run local:operator:image:load -- --clusters "${clusters}" --image "${image}"
run_step "Deploy Operator" mise run local:operator:deploy -- --clusters "${clusters}" --image "${image}"

for cluster in "${cluster_names[@]}"; do
  run_step "Operator Pods (${cluster})" kubectl_cluster "${cluster}" -n polykube-system get pods -o wide
  args="$(kubectl_cluster "${cluster}" -n polykube-system get deploy polykube-operator -o jsonpath='{.spec.template.spec.containers[0].args}')"
  printf 'operator args (%s): %s\n' "${cluster}" "${args}"
  assert_contains "${args}" "--cluster-member-name=${cluster}" "Operator deployment has incorrect cluster member identity for ${cluster}."
done

run_step "Apply Demo Manifests" mise run local:demo:apply -- --clusters "${clusters}"

for cluster in "${cluster_names[@]}"; do
  printf '\n## Verify ClusterMember (%s)\n' "${cluster}"
  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Ready")].status}' get clustermember "${cluster}"
  kubectl_cluster "${cluster}" get clustermember "${cluster}" -o yaml
done

printf '\n## Verify Federation\n'
wait_jsonpath_equals "${source_cluster}" "${#cluster_names[@]}" '{.status.readyMembers}' get federation local-dev
wait_jsonpath_equals "${source_cluster}" "True" '{.status.conditions[?(@.type=="Ready")].status}' get federation local-dev
kubectl_cluster "${source_cluster}" get federation local-dev -o yaml

for cluster in "${cluster_names[@]}"; do
  printf '\n## Verify Workload (%s)\n' "${cluster}"
  wait_resource_exists "${cluster}" -n default get deploy echo
  run_step "Deployment Rollout (${cluster})" kubectl_cluster "${cluster}" -n default rollout status deployment/echo --timeout=120s
  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="RuntimeObjectsApplied")].status}' -n default get workload echo
  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Available")].status}' -n default get workload echo
  kubectl_cluster "${cluster}" -n default get workload echo -o yaml
  kubectl_cluster "${cluster}" -n default get deploy echo -o wide
  kubectl_cluster "${cluster}" -n default get pods -l polykube.dev/workload=echo -o wide

  printf '\n## Verify Service Annotations (%s)\n' "${cluster}"
  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Ready")].status}' -n default get serviceendpoint echo
  kubectl_cluster "${cluster}" -n default get serviceendpoint echo -o yaml
  wait_jsonpath_equals "${cluster}" "true" '{.metadata.annotations.service\.cilium\.io/global}' -n default get svc echo
  wait_jsonpath_equals "${cluster}" "true" '{.metadata.annotations.service\.cilium\.io/shared}' -n default get svc echo
  kubectl_cluster "${cluster}" -n default get svc echo -o yaml
done

printf '\n## Verify Aggregate Workload Status\n'
status_contexts="$(cluster_context_for "${source_cluster}"),$(cluster_context_for "${destination_cluster}")"
aggregate_table="$(cd "${REPO_ROOT}/operator" && go run ./cmd/polykube-status --contexts "${status_contexts}" --namespace default)"
printf '%s\n' "${aggregate_table}"
assert_contains "${aggregate_table}" "$(cluster_context_for "${source_cluster}")" "Aggregate table is missing the source context."
assert_contains "${aggregate_table}" "$(cluster_context_for "${destination_cluster}")" "Aggregate table is missing the destination context."
assert_contains "${aggregate_table}" "Available" "Aggregate table is missing available target state."

aggregate_json="$(cd "${REPO_ROOT}/operator" && go run ./cmd/polykube-status --contexts "${status_contexts}" --namespace default --output json)"
printf '%s\n' "${aggregate_json}"
assert_contains "${aggregate_json}" '"context": "'"$(cluster_context_for "${source_cluster}")"'"' "Aggregate JSON is missing the source context."
assert_contains "${aggregate_json}" '"clusterMember": "'"${destination_cluster}"'"' "Aggregate JSON is missing the destination member."
assert_contains "${aggregate_json}" '"targetState": "Available"' "Aggregate JSON is missing available target state."

printf '\n## Verify DatastoreBinding Missing-Secret Recovery\n'
for cluster in "${cluster_names[@]}"; do
  kubectl_cluster "${cluster}" -n default delete datastorebinding primary --ignore-not-found --wait=true --timeout=120s
  kubectl_cluster "${cluster}" -n default delete secret echo-database --ignore-not-found
  kubectl_cluster "${cluster}" apply -f - <<'MANIFEST'
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

  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Degraded")].status}' -n default get datastorebinding primary
  wait_jsonpath_equals "${cluster}" "ConnectionSecretNotFound" '{.status.conditions[?(@.type=="Degraded")].reason}' -n default get datastorebinding primary

  kubectl_cluster "${cluster}" apply -f - <<'MANIFEST'
apiVersion: v1
kind: Secret
metadata:
  name: echo-database
  namespace: default
type: Opaque
stringData:
  url: postgres://polykube@database.default.svc:5432/echo
MANIFEST

  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Ready")].status}' -n default get datastorebinding primary
  wait_jsonpath_equals "${cluster}" "echo-database" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_URL")].valueFrom.secretKeyRef.name}' -n default get deployment echo
  wait_jsonpath_equals "${cluster}" "url" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_URL")].valueFrom.secretKeyRef.key}' -n default get deployment echo
  wait_jsonpath_equals "${cluster}" "None" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATASTORE_PRIMARY_REPLICATION_MODE")].value}' -n default get deployment echo
  wait_jsonpath_equals "${cluster}" "echo-database" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.name}' -n default get deployment echo
  wait_jsonpath_equals "${cluster}" "url" '{.spec.template.spec.containers[?(@.name=="app")].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.key}' -n default get deployment echo
  run_step "Deployment Rollout After Datastore Injection (${cluster})" kubectl_cluster "${cluster}" -n default rollout status deployment/echo --timeout=120s
  kubectl_cluster "${cluster}" -n default get datastorebinding primary -o yaml
  kubectl_cluster "${cluster}" -n default get deployment echo -o yaml
done

printf '\n## Verify Target-Policy Exclusion\n'
for cluster in "${cluster_names[@]}"; do
  kubectl_cluster "${cluster}" -n default delete workload target-policy-check --ignore-not-found --wait=true --timeout=120s
  kubectl_cluster "${cluster}" -n default delete deployment,service target-policy-check --ignore-not-found
  kubectl_cluster "${cluster}" apply -f - <<MANIFEST
apiVersion: runtime.polykube.dev/v1alpha1
kind: Workload
metadata:
  name: target-policy-check
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
      - ${destination_cluster}
MANIFEST
done

wait_jsonpath_equals "${source_cluster}" "Pending" '{.status.targets[0].state}' -n default get workload target-policy-check
wait_jsonpath_equals "${source_cluster}" "ExcludedByTargetPolicy" '{.status.conditions[?(@.type=="Pending")].reason}' -n default get workload target-policy-check
assert_resource_absent "${source_cluster}" -n default get deployment target-policy-check
assert_resource_absent "${source_cluster}" -n default get service target-policy-check
wait_resource_exists "${destination_cluster}" -n default get deployment target-policy-check
wait_resource_exists "${destination_cluster}" -n default get service target-policy-check
kubectl_cluster "${source_cluster}" -n default get workload target-policy-check -o yaml
kubectl_cluster "${destination_cluster}" -n default get workload target-policy-check -o yaml

printf '\n## Cross-Cluster HTTP Probe\n'
kubectl_cluster "${source_cluster}" -n default delete pod polykube-release-probe --ignore-not-found >/dev/null
kubectl_cluster "${source_cluster}" -n default run polykube-release-probe --rm -i --restart=Never \
  --image=curlimages/curl:8.12.1 -- curl -fsS --max-time 10 http://echo:5678

printf '\n## Verify Active-Passive Service Annotations\n'
for cluster in "${cluster_names[@]}"; do
  kubectl_cluster "${cluster}" -n default patch serviceendpoint echo --type=merge \
    -p "{\"spec\":{\"routingMode\":\"ActivePassive\",\"primaryMemberRef\":\"${source_cluster}\"}}"
  wait_jsonpath_equals "${cluster}" "True" '{.status.conditions[?(@.type=="Ready")].status}' -n default get serviceendpoint echo
  wait_jsonpath_equals "${cluster}" "${source_cluster}" '{.status.activeMemberRef}' -n default get serviceendpoint echo
  wait_jsonpath_equals "${cluster}" "true" '{.metadata.annotations.service\.cilium\.io/global}' -n default get svc echo
done
wait_jsonpath_equals "${source_cluster}" "true" '{.metadata.annotations.service\.cilium\.io/shared}' -n default get svc echo
wait_jsonpath_equals "${destination_cluster}" "false" '{.metadata.annotations.service\.cilium\.io/shared}' -n default get svc echo
kubectl_cluster "${source_cluster}" -n default get svc echo -o yaml
kubectl_cluster "${destination_cluster}" -n default get svc echo -o yaml

run_step "Render GitOps Operator Component" kubectl kustomize "${REPO_ROOT}/gitops/components/operator"

printf '\nSUMMARY status=pass evidence=%s\n' "${evidence_log}"
