# Real-Time Guitar Trainer Plan

## Summary
Build a Linux desktop V1 that feels like a rhythm-game guitar tab, but listens to a real guitar or bass. The UI is in Go and the DSP pitch detection is in Python. The Go app is the master clock and judge, while the Python process is the audio "ear". Communication is via a local Unix domain socket with newline-delimited JSON messages.

## Structured Summary Of Existing Notes
Goal
- A virtual-guitar feel with real instrument input and immediate feedback.
- Focus on guitar or bass first, add drums/keys later.

User Experience
- Rolling tab/fretboard that scrolls toward a hit line.
- Real-time pitch and timing judgement with adjustable slack.
- Feedback for hits, misses, and sustain accuracy.

Architecture
- Go "Face" handles UI, playhead, judging, and rendering.
- Python "Ear" handles audio capture and pitch detection.
- IPC via Unix domain socket for low latency.

Data Model
- Preprocessed song map stored as JSON with timestamps and target freqs.
- Notes include time, string, fret, frequency, and optional duration.

DSP Choices
- Aubio pitch detection, YIN or similar.
- Sample rate 44100 Hz, buffer 512, window 2048 (tunable).
- Confidence threshold and pitch tolerance in cents.

Timing And Judgement
- Constant analysis cadence.
- Time slack window plus pitch tolerance.
- Sustain notes scored continuously across duration.

## Decisions And Defaults
- Platform: Linux desktop.
- Stack: Go UI plus Python DSP, two processes.
- Scope: Guitar and bass only in V1.
- IPC: Unix domain sockets, newline-delimited JSON.
- Default tolerances: pitch +/- 25 cents, timing +/- 80 ms (tunable).

## Functional Requirements
- Load a song JSON and render a rolling tab at 60 FPS.
- Capture live audio, detect pitch in real time, and send to Go.
- Judge player input against timing and pitch windows.
- Provide visual feedback for accuracy.

## Non-Goals For V1
- Multi-instrument gameplay (drums/keys) beyond placeholders.
- Online features, multiplayer, or cloud storage.
- Full tab ingestion from Guitar Pro or PowerTab files.

## Implementation Plan
1. Define JSON schema and Go data structures.
2. Implement Go song loader and playhead clock.
3. Implement UI layout with 6 strings and hit line.
4. Implement note scroll math and rendering.
5. Implement IPC listener goroutine and message channel.
6. Implement judging logic and accuracy UI.
7. Implement Python ear process with Aubio and audio capture.
8. Add sample song JSON for smoke testing.
9. Document runtime settings and tunable parameters.

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

## Test Plan
- Parse valid and invalid song JSON.
- Verify note hits line at expected time.
- Verify IPC processing does not block rendering.
- Verify confidence threshold suppresses noise.
- Verify pitch tolerance accepts near-correct notes.
- Verify sustain scoring over duration.

## Risks And Mitigations
- Low-frequency pitch accuracy: increase window size or adaptive window.
- Latency spikes: increase buffer or prioritize audio thread.
- IPC jitter: use buffered channels and drop stale samples.

## Assumptions
- PipeWire is available on Linux for audio capture.
- User provides or generates a song JSON file.

## Open Items
- If you want Android or a single-language implementation, update this plan.
- If you want placeholders for drums or keys in V1, update the scope.
