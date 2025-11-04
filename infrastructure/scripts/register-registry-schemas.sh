#!/bin/sh
set -euo pipefail

SCHEMA_DIR=${SCHEMA_DIR:-/schemas}
SCHEMA_REGISTRY_URL=${SCHEMA_REGISTRY_URL:-http://schema-registry:8081}

# Ensure required tools are present (idempotent)
apk add --no-cache curl jq >/dev/null 2>&1 || true

echo "[schema-init] waiting for schema-registry at $SCHEMA_REGISTRY_URL..."
until curl -sS "$SCHEMA_REGISTRY_URL/subjects" >/dev/null; do sleep 1; done

echo "[schema-init] setting global compatibility"
curl -sS -X PUT "$SCHEMA_REGISTRY_URL/config" \
  -H 'Content-Type: application/vnd.schemaregistry.v1+json' \
  -d '{"compatibility":"NONE"}' >/dev/null

echo "[schema-init] registering subjects from $SCHEMA_DIR"
shopt_set=false
if command -v bash >/dev/null 2>&1; then
  # if bash exists, use nullglob to avoid literal *.json when none present
  set +e; . /etc/profile >/dev/null 2>&1 || true; set -e
fi

for f in "$SCHEMA_DIR"/*.json; do
  [ -e "$f" ] || continue
  s=$(basename "${f%.json}")
  payload=$(printf '{"schemaType":"JSON","schema":%s}' "$(jq -Rs . < "$f")")
  code=$(curl -sS -o /dev/null -w "%{http_code}" \
    -X POST "$SCHEMA_REGISTRY_URL/subjects/$s/versions" \
    -H 'Content-Type: application/vnd.schemaregistry.v1+json' \
    -d "$payload")
  if [ "$code" != "200" ] && [ "$code" != "201" ] && [ "$code" != "409" ]; then
    echo "[schema-init] failed to register $s (status $code)" >&2
    exit 1
  fi
  echo "[schema-init] registered $s (status $code)"
done

echo "[schema-init] done"
