#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/infrastructure/compose/docker-compose.yml"
COMPOSE_CMD=(docker compose -f "$COMPOSE_FILE")

cleanup() {
  echo "[smoke] tearing down compose stack"
  "${COMPOSE_CMD[@]}" down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT

export COMPOSE_DOCKER_CLI_BUILD=1
export DOCKER_BUILDKIT=1

SMOKE_SERVICES=(
  postgres
  postgres-migrate
  postgres-seed
  redis
  zookeeper
  kafka
  schema-registry
  identity-service
  activity-service
  activity-dlq-manager
  dgraph-alpha
  dgraph-schema
  exercise-ontology-service
  exercise-ontology-consumer
)

# shellcheck source=../lib/wait.sh disable=SC1091
. "$SCRIPT_DIR/../lib/wait.sh"

echo "[smoke] bringing up services: ${SMOKE_SERVICES[*]}"
"${COMPOSE_CMD[@]}" up --quiet-pull --build -d "${SMOKE_SERVICES[@]}"

assert_ready() {
  local label="$1"
  local fn="$2"
  local service="$3"
  local attempts="${4:-60}"
  if ! wait_for "$label" "$fn" "$attempts"; then
    echo "[smoke] $label failed readiness probe; recent logs:"
    "${COMPOSE_CMD[@]}" logs --no-color --tail=80 "$service" || true
    exit 1
  fi
}

assert_ready "postgres" pg_ready postgres 90
assert_ready "redis" redis_ready redis 90
assert_ready "kafka" kafka_ready kafka 90
assert_ready "schema-registry" schema_registry_ready schema-registry 90
assert_ready "identity-service" identity_ready identity-service 90
assert_ready "activity-service" activity_ready activity-service 90
assert_ready "dgraph" dgraph_ready dgraph-alpha 120
assert_ready "exercise-ontology-service" ontology_ready exercise-ontology-service 90
assert_ready "dlq-metrics" dlq_metrics_ready activity-dlq-manager 90
assert_ready "ontology-consumer-metrics" ontology_consumer_metrics_ready exercise-ontology-consumer 90

echo "[smoke] verifying schema registry subjects"
"$ROOT_DIR/scripts/ci/schema_registry_check.sh"

echo "[smoke] docker compose ps"
"${COMPOSE_CMD[@]}" ps

echo "[smoke] smoke test completed"
