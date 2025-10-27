#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

timestamp() {
  date +"%H:%M:%S"
}

log_status() {
  local label="$1"
  shift
  printf '[%s][smoke][%s] %s\n' "$(timestamp)" "$label" "$*" >&2
}

wait_for() {
  local label="$1"
  local fn="$2"
  local attempts="${3:-60}"
  local interval="${4:-2}"

  for ((i = 1; i <= attempts; i++)); do
    if "$fn"; then
      log_status "$label" "ready (attempt ${i}/${attempts})"
      return 0
    fi

    log_status "$label" "waiting (attempt ${i}/${attempts})"
    if (( i % 20 == 0 )); then
      log_status "$label" "capturing last logs for $fn"
      "${COMPOSE_CMD[@]}" logs --no-color --tail=20 | sed "s/^/[${label}] /" || true
    fi
    sleep "$interval"
  done

  log_status "$label" "timed out after ${attempts} attempts"
  return 1
}

compose_container_id() {
  local service="$1"
  "${COMPOSE_CMD[@]}" ps -q "$service" 2>/dev/null || true
}

service_health() {
  local service="$1"
  local cid
  cid=$(compose_container_id "$service")
  [[ -n "$cid" ]] || return 1
  docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid"
}

pg_ready() {
  local health
  health=$(service_health postgres) || return 1
  [[ "$health" == "healthy" ]]
}

redis_ready() {
  "${COMPOSE_CMD[@]}" exec -T redis redis-cli ping >/dev/null 2>&1
}

kafka_ready() {
  local cid state
  cid=$(compose_container_id kafka)
  [[ -n "$cid" ]] || return 1
  state=$(docker inspect --format '{{.State.Status}}' "$cid")
  [[ "$state" == "running" ]]
}

schema_registry_ready() {
  curl --silent --show-error --fail --max-time 3 http://localhost:8081/subjects >/dev/null
}

identity_ready() {
  curl --silent --show-error --fail --max-time 3 http://localhost:8000/healthz >/dev/null
}

activity_ready() {
  curl --silent --show-error --fail --max-time 3 http://localhost:8080/healthz >/dev/null
}
