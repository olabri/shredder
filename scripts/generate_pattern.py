import argparse
import json
import math
import random
import subprocess
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Note:
    time_ms: int
    string: int
    fret: int
    freq_hz: float
    duration_ms: int


NOTE_OFFSETS = {
    "C": 0,
    "C#": 1,
    "D": 2,
    "D#": 3,
    "E": 4,
    "F": 5,
    "F#": 6,
    "G": 7,
    "G#": 8,
    "A": 9,
    "A#": 10,
    "B": 11,
}


def base_frequencies(string_count: int) -> list[float]:
    if string_count == 4:
        return [98.0, 73.42, 55.0, 41.2]  # G2 D2 A1 E1
    return [329.63, 246.94, 196.0, 146.83, 110.0, 82.41]  # E4 B3 G3 D3 A2 E2


def generate_groove(
    bpm: float,
    difficulty: int,
    length_sec: int,
    string_count: int,
    seed: str | None,
) -> list[Note]:
    rng = random.Random()
    if seed is not None:
        try:
            rng.seed(int(seed))
        except ValueError:
            rng.seed(seed)

    beat_ms = 60000.0 / max(30.0, bpm)
    if difficulty <= 1:
        subdivision = 1
        fret_max = 3
        play_prob = 0.6
    elif difficulty == 2:
        subdivision = 2
        fret_max = 5
        play_prob = 0.7
    elif difficulty == 3:
        subdivision = 4
        fret_max = 9
        play_prob = 0.8
    else:
        subdivision = 4
        fret_max = 12
        play_prob = 0.85

    step_ms = beat_ms / subdivision
    total_steps = int((length_sec * 1000) / step_ms)
    base_freqs = base_frequencies(string_count)

    notes: list[Note] = []
    for step in range(total_steps):
        if rng.random() > play_prob:
            continue
        string = rng.randint(1, string_count)
        fret = rng.randint(0, fret_max)
        freq = base_freqs[string - 1] * math.pow(2.0, fret / 12.0)
        notes.append(
            Note(
                time_ms=int(step * step_ms),
                string=string,
                fret=fret,
                freq_hz=round(freq, 2),
                duration_ms=int(step_ms),
            )
        )

    if not notes:
        notes.append(
            Note(
                time_ms=0,
                string=1,
                fret=0,
                freq_hz=round(base_freqs[0], 2),
                duration_ms=int(step_ms),
            )
        )

    return notes


def mp3_duration_seconds(path: Path) -> float | None:
    try:
        result = subprocess.run(
            [
                "ffprobe",
                "-v",
                "error",
                "-show_entries",
                "format=duration",
                "-of",
                "default=nokey=1:noprint_wrappers=1",
                str(path),
            ],
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError):
        return None

    try:
        return float(result.stdout.strip())
    except ValueError:
        return None


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate a practice song pattern.")
    parser.add_argument("-o", "--out", required=True, help="output JSON path")
    parser.add_argument("--title", default=None, help="song title")
    parser.add_argument("--bpm", type=float, default=None, help="tempo in BPM")
    parser.add_argument("--difficulty", type=int, default=1, help="difficulty 1-4")
    parser.add_argument("--length", type=int, default=None, help="length in seconds")
    parser.add_argument("--strings", type=int, default=6, choices=[4, 6], help="string count")
    parser.add_argument("--seed", default=None, help="random seed")
    parser.add_argument("--mp3", type=str, default=None, help="mp3 input for duration defaults")

    args = parser.parse_args()

    title = args.title
    bpm = args.bpm if args.bpm is not None else 100.0
    length = args.length if args.length is not None else 20

    if args.mp3:
        mp3_path = Path(args.mp3)
        if title is None:
            title = mp3_path.stem
        duration = mp3_duration_seconds(mp3_path)
        if duration is not None and args.length is None:
            length = max(5, int(duration))
        elif duration is None:
            print("Warning: ffprobe not available or failed. Using default length.")

    if title is None:
        title = "Generated Groove"

    notes = generate_groove(
        bpm=bpm,
        difficulty=args.difficulty,
        length_sec=max(5, length),
        string_count=args.strings,
        seed=args.seed,
    )

    payload = {
        "title": title,
        "bpm": bpm,
        "sync_offset_ms": 0,
        "string_count": args.strings,
        "notes": [note.__dict__ for note in notes],
    }

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")

    print(f"Wrote {len(notes)} notes to {out_path}")


if __name__ == "__main__":
    main()
