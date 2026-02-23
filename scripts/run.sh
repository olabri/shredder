#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SONG_PATH=${1:-"$ROOT_DIR/assets/sample_song.json"}
HEADLESS=${HEADLESS:-0}
ASCII=${ASCII:-0}
FAKE=${FAKE:-0}
VERBOSE=${VERBOSE:-0}
ASCII_LOG_STDOUT=${ASCII_LOG_STDOUT:-0}
STRINGS=${STRINGS:-}
STRINGS_ARG=()
if [ -n "$STRINGS" ]; then
  STRINGS_ARG=(-strings "$STRINGS")
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go not found in PATH" >&2
  exit 1
fi

PYTHON_BIN=${PYTHON_BIN:-}
if [ -z "$PYTHON_BIN" ]; then
  for candidate in python3.11 python3.12 python3.10 python3.9 python3.8 python3; do
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

if [ -d "$ROOT_DIR/.venv" ]; then
  VENV_PY="$ROOT_DIR/.venv/bin/python"
  if [ -x "$VENV_PY" ]; then
    if ! "$VENV_PY" - <<'PY'
import sys
major, minor = sys.version_info[:2]
if (major, minor) >= (3, 14):
    sys.exit(1)
PY
    then
      rm -rf "$ROOT_DIR/.venv"
    fi
  else
    rm -rf "$ROOT_DIR/.venv"
  fi
fi

if [ ! -d "$ROOT_DIR/.venv" ]; then
  "$PYTHON_BIN" -m venv "$ROOT_DIR/.venv"
fi

# shellcheck disable=SC1091
source "$ROOT_DIR/.venv/bin/activate"

PIP_QUIET="--quiet"
if [ "$VERBOSE" = "1" ]; then
  PIP_QUIET=""
fi

pip install $PIP_QUIET --upgrade pip setuptools wheel
pip install $PIP_QUIET "numpy<2.0"
pip install $PIP_QUIET pyaudio
CFLAGS="${CFLAGS:-} -Wno-incompatible-function-pointer-types" \
  pip install $PIP_QUIET --no-build-isolation "aubio==0.4.9"

GOPATH=${GOPATH:-/tmp/go}
GOMODCACHE=${GOMODCACHE:-/tmp/go/pkg/mod}
GOCACHE=${GOCACHE:-/tmp/go-build}
SOCKET_PATH=${SOCKET_PATH:-/tmp/guitar_ear.sock}
LOG_DIR="$ROOT_DIR/tmp"
LOG_FILE="$LOG_DIR/face.log"
TAIL_PID=""

mkdir -p "$GOPATH" "$GOMODCACHE" "$GOCACHE" "$LOG_DIR"
rm -f "$SOCKET_PATH"
touch "$LOG_FILE"

LOG_TO_STDOUT=0
if [ "$VERBOSE" = "1" ]; then
  LOG_TO_STDOUT=1
fi
if [ "$ASCII" = "1" ] && [ "$ASCII_LOG_STDOUT" = "1" ]; then
  LOG_TO_STDOUT=1
fi

if [ "$ASCII" = "1" ]; then
  if [ "$FAKE" = "1" ] && [ -z "${EAR_FAKE:-}" ]; then
    export EAR_FAKE=1
  fi
  export SOCKET_PATH
  export EAR_CONNECT_RETRY_SEC=10

  # Start ear first; it will retry until ASCII server is ready.
  "$PYTHON_BIN" "$ROOT_DIR/python/ear.py" &
  EAR_PID=$!

  cleanup() {
    kill "$EAR_PID" >/dev/null 2>&1 || true
    if [ -n "$TAIL_PID" ]; then
      kill "$TAIL_PID" >/dev/null 2>&1 || true
    fi
  }
  trap cleanup EXIT

  if [ "$LOG_TO_STDOUT" = "1" ]; then
    GOPATH="$GOPATH" GOMODCACHE="$GOMODCACHE" GOCACHE="$GOCACHE" \
      go run "$ROOT_DIR/cmd/ascii" -song "$SONG_PATH" "${STRINGS_ARG[@]}" | tee "$LOG_FILE"
  else
    GOPATH="$GOPATH" GOMODCACHE="$GOMODCACHE" GOCACHE="$GOCACHE" \
      go run "$ROOT_DIR/cmd/ascii" -song "$SONG_PATH" "${STRINGS_ARG[@]}" >"$LOG_FILE" 2>&1
  fi
  exit 0
elif [ "$HEADLESS" = "1" ]; then
  ( \
    GOPATH="$GOPATH" GOMODCACHE="$GOMODCACHE" GOCACHE="$GOCACHE" \
    go run "$ROOT_DIR/cmd/headless" \
  ) >"$LOG_FILE" 2>&1 &
else
  ( \
    GOPATH="$GOPATH" GOMODCACHE="$GOMODCACHE" GOCACHE="$GOCACHE" \
    go run "$ROOT_DIR/cmd/face" -song "$SONG_PATH" "${STRINGS_ARG[@]}" \
  ) >"$LOG_FILE" 2>&1 &
fi
FACE_PID=$!
TAIL_PID=""

cleanup() {
  kill "$FACE_PID" >/dev/null 2>&1 || true
  if [ -n "$TAIL_PID" ]; then
    kill "$TAIL_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [ "$LOG_TO_STDOUT" = "1" ]; then
  tail -n +1 -f "$LOG_FILE" &
  TAIL_PID=$!
fi

for _ in $(seq 1 50); do
  if [ -S "$SOCKET_PATH" ]; then
    break
  fi
  sleep 0.1
done

if [ ! -S "$SOCKET_PATH" ]; then
  echo "Go UI failed to create socket at $SOCKET_PATH." >&2
  echo "See log: $LOG_FILE" >&2
  cat "$LOG_FILE" >&2 || true
  exit 1
fi

if [ "$FAKE" = "1" ] && [ -z "${EAR_FAKE:-}" ]; then
  export EAR_FAKE=1
fi

if ! "$PYTHON_BIN" - <<'PY'
import socket, sys, os
path = os.environ.get("SOCKET_PATH", "/tmp/guitar_ear.sock")
try:
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.settimeout(0.5)
    s.connect(path)
    s.close()
except Exception:
    sys.exit(1)
PY
then
  echo "Socket exists but is not accepting connections: $SOCKET_PATH" >&2
  echo "See log: $LOG_FILE" >&2
  cat "$LOG_FILE" >&2 || true
  exit 1
fi

SOCKET_PATH="$SOCKET_PATH" "$PYTHON_BIN" "$ROOT_DIR/python/ear.py"
