#!/usr/bin/env sh
set -eu

SCHEMA_FILE=${SCHEMA_FILE:-/schema/exercise.schema}
DGRAPH_URL=${DGRAPH_URL:-http://dgraph-alpha:8080/alter}

if [ ! -f "$SCHEMA_FILE" ]; then
  echo "schema file $SCHEMA_FILE not found" >&2
  exit 1
fi

echo "Applying Dgraph schema from $SCHEMA_FILE to $DGRAPH_URL"
response=$(curl -sS -w "\n%{http_code}" -X POST "$DGRAPH_URL" -d "$(cat "$SCHEMA_FILE")")
status=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
if [ "$status" -ge 300 ]; then
  echo "Failed to apply schema: HTTP $status" >&2
  echo "$body" >&2
  exit 1
fi

echo "Dgraph schema applied successfully"
