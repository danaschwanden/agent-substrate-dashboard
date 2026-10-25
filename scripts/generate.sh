#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
go_output="${repo_dir}/backend/internal/httpapi/generated_types.go"
go_server_output="${repo_dir}/backend/internal/httpapi/generated_server.go"
ts_output="${repo_dir}/frontend/src/generated/api.ts"
temp_dir=""

if [[ "${1:-}" == "--check" ]]; then
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' EXIT
  go_output="${temp_dir}/generated_types.go"
  go_server_output="${temp_dir}/generated_server.go"
  ts_output="${temp_dir}/api.ts"
fi

mkdir -p "$(dirname -- "${go_output}")" "$(dirname -- "${ts_output}")"

(
  cd "${repo_dir}/backend"
  go tool oapi-codegen \
    -generate types \
    -package httpapi \
    -o "${go_output}" \
    ../api/openapi.yaml
)

(
  cd "${repo_dir}/backend"
  go tool oapi-codegen \
    -generate std-http \
    -package httpapi \
    -o "${go_server_output}" \
    ../api/openapi.yaml
)

(
  cd "${repo_dir}/frontend"
  npm exec -- openapi-typescript ../api/openapi.yaml -o "${ts_output}"
)

if [[ -n "${temp_dir}" ]]; then
  cmp "${go_output}" "${repo_dir}/backend/internal/httpapi/generated_types.go"
  cmp "${go_server_output}" "${repo_dir}/backend/internal/httpapi/generated_server.go"
  cmp "${ts_output}" "${repo_dir}/frontend/src/generated/api.ts"
fi
