#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

pushd "$ROOT_DIR" >/dev/null
trap 'popd >/dev/null' EXIT

export TESTCONTAINERS_RYUK_DISABLED=${TESTCONTAINERS_RYUK_DISABLED:-true}
export TESTCONTAINERS_CHECK_DUPLICATE=true

services=("activity-service" "exercise-ontology-service")

for svc in "${services[@]}"; do
  pushd "services/${svc}" >/dev/null
  if [[ "${svc}" == "activity-service" ]]; then
    go test -tags=integration ./internal/... -v
  else
    go test -tags=integration ./... -v
  fi
  popd >/dev/null
done
