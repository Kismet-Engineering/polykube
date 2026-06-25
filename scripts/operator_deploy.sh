#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
image="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:dev}"
context=""
kubeconfig=""
cluster_member_name=""
wait="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image) image="$2"; shift 2 ;;
    --context) context="$2"; shift 2 ;;
    --kubeconfig) kubeconfig="$2"; shift 2 ;;
    --cluster-member-name) cluster_member_name="$2"; shift 2 ;;
    --wait) wait="$2"; shift 2 ;;
    *) shift ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

require_cmd kubectl

kubectl_args=()
if [[ -n "${kubeconfig}" ]]; then
  kubectl_args+=(--kubeconfig "${kubeconfig}")
fi
if [[ -n "${context}" ]]; then
  kubectl_args+=(--context "${context}")
fi

kubectl "${kubectl_args[@]}" apply -f "${repo_root}/operator/config/crd/bases"
render_args=(--image "${image}")
if [[ -n "${cluster_member_name}" ]]; then
  render_args+=(--cluster-member-name "${cluster_member_name}")
fi
"${repo_root}/scripts/operator_render.sh" "${render_args[@]}" | kubectl "${kubectl_args[@]}" apply -f -

if [[ "${wait}" == "true" ]]; then
  kubectl "${kubectl_args[@]}" -n polykube-system rollout status deployment/polykube-operator --timeout=120s
fi
