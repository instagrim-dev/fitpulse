#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA_DIR="$ROOT_DIR/schemas/registry"
SCHEMA_REGISTRY_URL="${SCHEMA_REGISTRY_URL:-http://localhost:8081}"

set_compatibility() {
  local response status
  response=$(mktemp)
  status=$(curl -sS -o "$response" -w "%{http_code}" \
    -X PUT "$SCHEMA_REGISTRY_URL/config" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d '{"compatibility":"BACKWARD_TRANSITIVE"}')

  if [[ "$status" -ge 400 ]]; then
    echo "[schema] failed to set global compatibility (status $status):"
    cat "$response" >&2
    rm -f "$response"
    exit 1
  fi

  rm -f "$response"
}

register_schema() {
  local file="$1"
  local subject
  subject="$(basename "${file%.json}")"

  echo "[schema] registering subject $subject from $(basename "$file")"

  local payload status response
  payload=$(python3 - "$file" <<'PY'
import json, sys, pathlib
schema_path = pathlib.Path(sys.argv[1])
with schema_path.open() as fh:
    schema = json.load(fh)
payload = {
    "schemaType": "JSON",
    "schema": json.dumps(schema, separators=(",", ":"))
}
json.dump(payload, sys.stdout, separators=(",", ":"))
PY
)

  response=$(mktemp)
  status=$(curl -sS -o "$response" -w "%{http_code}" \
    -X POST "$SCHEMA_REGISTRY_URL/subjects/$subject/versions" \
    -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    -d "$payload")

  # 200 OK -> success, 409 Conflict -> schema already registered (acceptable)
  if [[ "$status" != "200" && "$status" != "201" && "$status" != "409" ]]; then
    echo "[schema] failed to register $subject (status $status):" >&2
    cat "$response" >&2
    rm -f "$response"
    exit 1
  fi

  rm -f "$response"
}

main() {
  if [[ ! -d "$SCHEMA_DIR" ]]; then
    echo "[schema] directory $SCHEMA_DIR not found" >&2
    exit 1
  fi

  set_compatibility

  local file
  shopt -s nullglob
  for file in "$SCHEMA_DIR"/*.json; do
    register_schema "$file"
  done
  shopt -u nullglob
}

main "$@"
