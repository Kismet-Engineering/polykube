#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
context=""
kubeconfig=""
delete_crds="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --context) context="$2"; shift 2 ;;
    --kubeconfig) kubeconfig="$2"; shift 2 ;;
    --crds) delete_crds="$2"; shift 2 ;;
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

"${repo_root}/scripts/operator_render.sh" | kubectl "${kubectl_args[@]}" delete --ignore-not-found -f -
if [[ "${delete_crds}" == "true" ]]; then
  kubectl "${kubectl_args[@]}" delete --ignore-not-found -f "${repo_root}/operator/config/crd/bases"
fi
