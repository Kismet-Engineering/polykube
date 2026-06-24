#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
image="${POLYKUBE_OPERATOR_IMAGE:-polykube-operator:dev}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image) image="$2"; shift 2 ;;
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

mkdir -p "${repo_root}/tmp"
tmp_dir="$(mktemp -d "${repo_root}/tmp/operator-render.XXXXXX")"
trap 'rm -rf "${tmp_dir}"' EXIT

image_name="${image}"
image_tag="latest"
last_path_segment="${image##*/}"
if [[ "${last_path_segment}" == *:* ]]; then
  image_name="${image%:*}"
  image_tag="${image##*:}"
fi

cat >"${tmp_dir}/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../gitops/components/operator
images:
  - name: polykube-operator
    newName: ${image_name}
    newTag: ${image_tag}
EOF

kubectl kustomize "${tmp_dir}"
