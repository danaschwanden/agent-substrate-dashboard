#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
mkdir -p "${repo_dir}/.cache/go-build"

(
  cd "${repo_dir}/backend"
  GOCACHE="${repo_dir}/.cache/go-build" go test ./...
)
