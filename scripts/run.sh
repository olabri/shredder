#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SONG_PATH=${1:-"$ROOT_DIR/assets/sample_song.json"}

if ! command -v go >/dev/null 2>&1; then
  echo "go not found in PATH" >&2
  exit 1
fi

PYTHON_BIN=${PYTHON_BIN:-}
if [ -z "$PYTHON_BIN" ]; then
  for candidate in python3.12 python3.11 python3.10 python3.9 python3.8 python3; do
    if command -v "$candidate" >/dev/null 2>&1; then
      PYTHON_BIN=$candidate
      break
    fi
  done
fi

if [ -z "$PYTHON_BIN" ]; then
  echo "No compatible Python found (tried python3.12 through python3)." >&2
  exit 1
fi

if ! "$PYTHON_BIN" - <<'PY'
import sys
major, minor = sys.version_info[:2]
if (major, minor) >= (3, 14):
    sys.exit(1)
PY
then
  echo "Python 3.14+ detected. aubio fails to build there. Please install Python 3.12 or 3.11." >&2
  exit 1
fi

if [ ! -d "$ROOT_DIR/.venv" ]; then
  "$PYTHON_BIN" -m venv "$ROOT_DIR/.venv"
fi

# shellcheck disable=SC1091
source "$ROOT_DIR/.venv/bin/activate"

pip install --quiet --upgrade pip
pip install --quiet -r "$ROOT_DIR/python/requirements.txt"

GOPATH=${GOPATH:-/tmp/go}
GOMODCACHE=${GOMODCACHE:-/tmp/go/pkg/mod}
GOCACHE=${GOCACHE:-/tmp/go-build}

mkdir -p "$GOPATH" "$GOMODCACHE" "$GOCACHE"

( \
  GOPATH="$GOPATH" GOMODCACHE="$GOMODCACHE" GOCACHE="$GOCACHE" \
  go run "$ROOT_DIR/cmd/face" "$SONG_PATH" 
) &
FACE_PID=$!

cleanup() {
  kill "$FACE_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

"$PYTHON_BIN" "$ROOT_DIR/python/ear.py"
