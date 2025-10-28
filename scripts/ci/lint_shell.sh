#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

mapfile -d '' files < <(
  find "$ROOT_DIR/scripts" "$ROOT_DIR/infrastructure" -type f -name '*.sh' -print0
)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "[shell lint] no shell scripts found."
  exit 0
fi

if command -v shellcheck >/dev/null 2>&1; then
  shellcheck "${files[@]}"
  exit 0
fi

files_rel=()
for file in "${files[@]}"; do
  files_rel+=("${file#"$ROOT_DIR"/}")
done

docker run --rm \
  -v "$ROOT_DIR":/workspace \
  -w /workspace \
  koalaman/shellcheck:stable \
  "${files_rel[@]}"
