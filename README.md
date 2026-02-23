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

Select 4 vs 6 strings:
```sh
STRINGS=4 ASCII=1 ./scripts/run.sh
```

Or for the UI:
```sh
STRINGS=4 ./scripts/run.sh
```

ASCII UI with live stdout (not just tmp/face.log):
```sh
ASCII=1 ASCII_LOG_STDOUT=1 ./scripts/run.sh
```

Verbose logs for any mode:
```sh
VERBOSE=1 ./scripts/run.sh
```

## Controls
Go UI:
- `Space` pause/resume
- `S` stop/reset
- `+` / `-` or numpad `+` / `-` adjust speed (0.5x–2.0x)

ASCII UI:
- `p` toggle pause/resume
- `s` stop/reset
- `+` / `-` adjust speed (0.5x–2.0x)
- `q` quit

## Creating Patterns And MP3 Analysis
See `docs/patterns-and-mp3.md` for the full guide.

## Pattern Generator (CLI)
Generate a practice pattern (algorithmic mode):
```sh
python3 ./scripts/generate_pattern.py -o ./assets/generated_song.json --bpm 110 --difficulty 2 --length 30 --strings 6 --seed 42
```

Then run it:
```sh
./scripts/run.sh ./assets/generated_song.json
```

Use an MP3 to default the song length (requires `ffprobe`):
```sh
python3 ./scripts/generate_pattern.py --mp3 ./assets/example.mp3 -o ./assets/generated_song.json --bpm 110 --difficulty 2 --strings 6
```

## Hair Metal Example (PCM)
Generate the synthetic hair metal track and lyrics:
```sh
python3 scripts/generate_hair_metal_pcm.py
```

Outputs:
- `assets/hair_metal.pcm` (raw PCM, 44.1kHz, stereo)
- `assets/hair_metal.wav` (PCM in a WAV container)

Lyrics and structure:
- `docs/hair-metal-song.md`

Manual two-terminal setup (prefer Python 3.12/3.11):
1. Start the Go UI:
```sh
go run ./cmd/face ./assets/sample_song.json
```

2. In another terminal, start the Python ear:
```sh
python3 ./python/ear.py
```

Sample bass song:
```sh
STRINGS=4 ./scripts/run.sh ./assets/sample_bass.json
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
