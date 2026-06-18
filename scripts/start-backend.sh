#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PORT="${1:-${PORT:-3000}}"

cd "$PROJECT_ROOT"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or is not on PATH." >&2
  exit 1
fi

echo "Starting RSP backend on ws://localhost:${PORT}"
echo "Press Ctrl+C to stop."

PORT="$PORT" go run .
