#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

pushd "$ROOT_DIR" >/dev/null
trap 'popd >/dev/null' EXIT

cd services/activity-service

export TESTCONTAINERS_RYUK_DISABLED=${TESTCONTAINERS_RYUK_DISABLED:-true}
export TESTCONTAINERS_CHECK_DUPLICATE=true

go test -tags=integration ./internal/outbox -v
