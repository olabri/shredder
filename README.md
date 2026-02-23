# shredder

Real-time guitar trainer prototype. Go renders a rolling tab and judges timing and pitch. A Python process detects pitch from live audio and streams it to Go over a Unix domain socket.

## Requirements
- Go 1.22+
- Python 3.12 or 3.11 recommended (aubio build support)
- PipeWire or ALSA-compatible input

Python dependencies:
- `aubio`
- `numpy`
- `pyaudio`

## Run (Linux)
One command:
```sh
./scripts/run.sh
```

Or with Make:
```sh
make run
```

Headless IPC smoke test (no UI):
```sh
HEADLESS=1 EAR_FAKE=1 ./scripts/run.sh
```

Or:
```sh
make headless
```

ASCII UI (terminal-based):
```sh
ASCII=1 ./scripts/run.sh
```

Or:
```sh
make ascii
```

ASCII UI with live stdout (not just tmp/face.log):
```sh
ASCII=1 ASCII_LOG_STDOUT=1 ./scripts/run.sh
```

Verbose logs for any mode:
```sh
VERBOSE=1 ./scripts/run.sh
```

Manual two-terminal setup (prefer Python 3.12/3.11):
1. Start the Go UI:
```sh
go run ./cmd/face ./assets/sample_song.json
```

2. In another terminal, start the Python ear:
```sh
python3 ./python/ear.py
```

Press `Space` in the Go window to pause or resume.

## Tunables
Edit these constants in `cmd/face/main.go`:
- `pitchToleranceC`
- `timingSlackMs`
- `confidenceMin`
- `scrollSpeedPxMs`

Edit these constants in `python/ear.py`:
- `BUFFER_SIZE`
- `WINDOW_SIZE`
- `SAMPLE_RATE`
- `CONFIDENCE_MIN`
