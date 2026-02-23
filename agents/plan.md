# Project Plan: Real-Time Guitar Trainer

## Summary
Build a Linux desktop V1 that feels like a rhythm-game guitar tab while listening to a real guitar or bass. The Go app renders the UI and judges timing and pitch. A Python process detects pitch in real time and streams results to Go via a Unix domain socket.

## Structured Summary Of Conversation
Goal
- Real-instrument input with a "virtual guitar" feel and immediate feedback.
- Start with guitar/bass, add drums or keys later.

User Experience
- Rolling tab/fretboard with a hit line.
- Pitch and timing judgement with adjustable slack.
- Visual feedback for hits, misses, and sustain accuracy.

Architecture
- Go "Face": UI, playhead/transport, judging, rendering.
- Python "Ear": audio capture and pitch detection.
- IPC via Unix domain sockets with newline-delimited JSON.

Data Model
- Preprocessed song map in JSON: timestamped notes with string, fret, freq, and optional duration.

DSP Choices
- Aubio pitch detection with confidence gating.
- Sample rate 44100 Hz, buffer 512, window 2048 (tunable).

Judgement
- Constant analysis cadence.
- Time slack window and pitch tolerance in cents.
- Sustain notes scored continuously across duration.

## Decisions And Defaults
- Platform: Linux desktop.
- Stack: Go UI + Python DSP.
- Scope: Guitar and bass only in V1.
- IPC: Unix domain sockets, newline-delimited JSON.
- Default tolerances: pitch +/- 25 cents, timing +/- 80 ms.

## Requirements
- Load song JSON and render a rolling tab at 60 FPS.
- Capture live audio and detect pitch in real time.
- Judge player input against timing and pitch windows.
- Provide visual accuracy feedback.

## Non-Goals For V1
- Multi-instrument gameplay beyond placeholders.
- Online features or cloud storage.
- Full Guitar Pro or PowerTab ingestion.

## Implementation Plan
1. Define JSON schema and Go data structures.
2. Implement Go song loader and playhead clock.
3. Implement UI layout with 6 strings and hit line.
4. Implement note scroll math and rendering.
5. Implement IPC listener goroutine and message channel.
6. Implement judging logic and accuracy UI.
7. Implement Python ear process with Aubio and audio capture.
8. Add sample song JSON for smoke testing.
9. Document tunable parameters.

## Public Interfaces
Song JSON
- title: string
- bpm: number (optional)
- sync_offset_ms: integer
- notes: array of note objects

Note object
- time_ms: integer
- string: integer (1-6)
- fret: integer (>= 0)
- freq_hz: number
- duration_ms: integer (optional)

IPC message (newline-delimited JSON)
- freq: number
- conf: number
- ts_ms: integer (optional)

## Tests
- Parse valid and invalid song JSON.
- Verify note hits line at expected time.
- Verify IPC processing does not block rendering.
- Verify confidence threshold suppresses noise.
- Verify pitch tolerance accepts near-correct notes.
- Verify sustain scoring over duration.

## Risks And Mitigations
- Low-frequency accuracy: increase window size or use adaptive windows.
- Latency spikes: raise buffer size or prioritize audio thread.
- IPC jitter: use buffered channels and drop stale samples.

## Assumptions
- PipeWire available on Linux for audio capture.
- User provides or generates a song JSON file.
