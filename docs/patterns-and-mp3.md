# Creating Patterns And MP3 Analysis

This guide explains how to create more song patterns and how to analyze MP3 files into the app’s JSON format.

## Ways To Create Patterns
1. Hand-author JSON
- Best accuracy, fastest validation.
- Start from a tab or MIDI and convert to `{time_ms, string, fret, freq_hz}`.

2. Algorithmic generators
- Best for drills, warmups, and difficulty scaling.
- Generates predictable practice patterns without any audio analysis.

3. MP3 analysis
- Automatic transcription at scale.
- Requires cleanup; polyphonic mixes are noisy.

## Practical MP3 Pipeline
1. Decode MP3 to PCM/WAV (mono, fixed sample rate)
2. Estimate tempo and beat grid
3. Detect note onsets
4. Track pitch and segment into notes
5. For polyphonic mixes, separate stems first
6. Map pitch to string/fret (tuning heuristics)
7. Export JSON and validate in the UI

## CLI Generator (Algorithmic)
Create a groove and optionally derive duration from an MP3:
```sh
python3 scripts/generate_pattern.py -o ./assets/generated_song.json --bpm 110 --difficulty 2 --length 30 --strings 6 --seed 42
```

Use MP3 duration defaults (requires `ffprobe`):
```sh
python3 scripts/generate_pattern.py --mp3 ./assets/example.mp3 -o ./assets/generated_song.json --bpm 110 --difficulty 2 --strings 6
```

## Decode Example
```sh
ffmpeg -i input.mp3 -ac 1 -ar 44100 output.wav
```

## Caveats
- Mixed, polyphonic MP3s are noisy. Stem separation improves results.
- Beat tracking can drift in complex sections; expect manual correction.
- Pitch tracking is easier on isolated bass/guitar stems.

## Suggested Tools
- Audio decode: ffmpeg
- Beat/onset/pitch: librosa or aubio
- Polyphonic separation: Demucs or Spleeter
