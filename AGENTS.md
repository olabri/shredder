# AGENTS.md

## Project Overview
This repo plans a real-time guitar trainer with a rhythm-game feel. A Go desktop UI renders a rolling tab and judges timing, while a Python process detects pitch from live audio input.

## Core Stack
- UI: Go (Ebitengine recommended)
- DSP: Python with Aubio
- IPC: Unix domain socket, newline-delimited JSON
- Platform: Linux desktop (PipeWire preferred)

## Conventions
- Use ASCII in source files unless there is a clear need for Unicode.
- Keep JSON schemas stable once introduced.
- Avoid hidden cross-process state; all shared state should be explicit in IPC messages.
- Do not block the render loop on I/O.

## Data Contracts
- Song data is JSON with a list of timestamped note events.
- IPC messages must be newline-delimited JSON containing frequency and confidence.

## Quality Bars
- UI must remain responsive at 60 FPS while listening to pitch data.
- Audio analysis should tolerate noise via confidence thresholds and pitch windows.
- Timing judgement should allow configurable slack windows.

## Testing
- Unit test JSON parsing and timing math.
- Manual smoke test with a short scale and live instrument input.

## When Unsure
- Prefer a simpler implementation with clear tuning parameters over heavy heuristics.
- Ask for clarification if new features change the data contract.
