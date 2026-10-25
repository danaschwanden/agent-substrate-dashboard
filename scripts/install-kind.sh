#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
overlay_dir="${repo_dir}/deploy/kustomize/overlays/.generated-kind"

kind_cluster_name="${KIND_CLUSTER_NAME:-kind}"
kube_context="${KUBE_CONTEXT:-kind-${kind_cluster_name}}"
atespace="${DASHBOARD_ATESPACE:-}"
worker_pool="${DASHBOARD_WORKER_POOL:-}"
image_source="${DASHBOARD_IMAGE_SOURCE:-local}"
image_owner="${DASHBOARD_IMAGE_OWNER:-agent-substrate}"
image_tag="${DASHBOARD_IMAGE_TAG:-v0.1.0}"
enable_operations="${DASHBOARD_ENABLE_OPERATIONS:-false}"

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_command kubectl

[[ -n "${atespace}" ]] || fail "set DASHBOARD_ATESPACE to the atespace shown by: kubectl ate get atespaces"
[[ -n "${worker_pool}" ]] || fail "set DASHBOARD_WORKER_POOL to a pool shown by: kubectl get workerpools.ate.dev -A"
[[ "${image_source}" == "local" || "${image_source}" == "ghcr" ]] || fail "DASHBOARD_IMAGE_SOURCE must be local or ghcr"
[[ "${enable_operations}" == "true" || "${enable_operations}" == "false" ]] || fail "DASHBOARD_ENABLE_OPERATIONS must be true or false"

current_context="$(kubectl config current-context 2>/dev/null || true)"
[[ "${current_context}" == "${kube_context}" ]] || fail "current context is '${current_context:-none}', expected '${kube_context}'; run: kubectl config use-context ${kube_context}"

printf 'Checking Agent Substrate on %s...\n' "${kube_context}"
kubectl --context "${kube_context}" -n ate-system get service api >/dev/null
kubectl --context "${kube_context}" get clustertrustbundles \
  -l podcert.ate.dev/canarying=live -o name | grep -q . \
  || fail "no live Agent Substrate ClusterTrustBundle was found"

if ! kubectl --context "${kube_context}" get workerpools.ate.dev -A \
  -o custom-columns=NAME:.metadata.name --no-headers | grep -Fxq "${worker_pool}"; then
  fail "WorkerPool '${worker_pool}' was not found; run: kubectl get workerpools.ate.dev -A"
fi

if [[ "${image_source}" == "local" ]]; then
  require_command docker
  require_command kind
  "${repo_dir}/scripts/check-kind-version.sh"
  docker info >/dev/null

  printf 'Building local dashboard images...\n'
  docker buildx build --load \
    --build-arg VERSION=development \
    -f "${repo_dir}/backend/Dockerfile" \
    -t agent-substrate-dashboard-backend:dev \
    "${repo_dir}"
  docker buildx build --load \
    --build-arg VERSION=development \
    -f "${repo_dir}/frontend/Dockerfile" \
    -t agent-substrate-dashboard-frontend:dev \
    "${repo_dir}"

  printf 'Loading images into kind cluster %s...\n' "${kind_cluster_name}"
  kind load docker-image --name "${kind_cluster_name}" \
    agent-substrate-dashboard-backend:dev \
    agent-substrate-dashboard-frontend:dev

  backend_image="agent-substrate-dashboard-backend"
  frontend_image="agent-substrate-dashboard-frontend"
  image_tag="dev"
else
  backend_image="ghcr.io/${image_owner}/agent-substrate-dashboard-backend"
  frontend_image="ghcr.io/${image_owner}/agent-substrate-dashboard-frontend"
fi

mkdir -p "${overlay_dir}" "${repo_dir}/.cache"
cat >"${overlay_dir}/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../base
configMapGenerator:
- name: agent-substrate-dashboard-config
  behavior: merge
  literals:
  - DASHBOARD_ATESPACE=${atespace}
  - DASHBOARD_WORKER_POOL=${worker_pool}
  - DASHBOARD_CONTROL_ENDPOINT=api.ate-system.svc:443
  - DASHBOARD_CONTROL_SERVER_NAME=api.ate-system.svc
  - DASHBOARD_ENABLE_OPERATIONS=${enable_operations}
images:
- name: ghcr.io/agent-substrate/agent-substrate-dashboard-backend
  newName: ${backend_image}
  newTag: ${image_tag}
- name: ghcr.io/agent-substrate/agent-substrate-dashboard-frontend
  newName: ${frontend_image}
  newTag: ${image_tag}
EOF

printf 'Rendering and applying the dashboard...\n'
kubectl --context "${kube_context}" kustomize "${overlay_dir}" >"${repo_dir}/.cache/dashboard-kind-rendered.yaml"
kubectl --context "${kube_context}" apply -k "${overlay_dir}"

kubectl --context "${kube_context}" -n agent-substrate-dashboard \
  rollout status deployment/agent-substrate-dashboard-backend --timeout=180s
kubectl --context "${kube_context}" -n agent-substrate-dashboard \
  rollout status deployment/agent-substrate-dashboard-frontend --timeout=180s

cat <<EOF

Dashboard workloads are ready.

Next steps:
  1. Grant AccessPolicy as documented in docs/KIND_QUICKSTART.md.
  2. Start browser access:
     kubectl -n agent-substrate-dashboard port-forward service/agent-substrate-dashboard-frontend 8090:80
  3. Open http://127.0.0.1:8090

The rendered manifest is at .cache/dashboard-kind-rendered.yaml.
EOF
