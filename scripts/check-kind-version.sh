#!/usr/bin/env bash
set -euo pipefail

minimum_version="v0.27.0"
reported_version="${1:-}"

if [[ -z "${reported_version}" ]]; then
  reported_version="$(kind version)"
fi

if [[ "${reported_version}" =~ (v?)([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
  major="${BASH_REMATCH[2]}"
  minor="${BASH_REMATCH[3]}"
  patch="${BASH_REMATCH[4]}"
else
  printf 'error: could not parse kind version from: %s\n' "${reported_version}" >&2
  exit 1
fi

if (( major == 0 && minor < 27 )); then
  cat >&2 <<EOF
error: kind v${major}.${minor}.${patch} is too old; ${minimum_version} or newer is required.
Older kind releases cannot load images into nodes using containerd 2.x and fail with:
  ERROR: failed to detect containerd snapshotter

Upgrade kind, then rerun the installer:
  macOS with Homebrew: brew upgrade kind
  Other platforms: https://kind.sigs.k8s.io/docs/user/quick-start/#installation
EOF
  exit 1
fi
