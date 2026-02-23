import argparse
import json
import math
import random
from dataclasses import dataclass


@dataclass
class Note:
    time_ms: int
    string: int
    fret: int
    freq_hz: float
    duration_ms: int


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


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate a practice song pattern.")
    parser.add_argument("-o", "--out", required=True, help="output JSON path")
    parser.add_argument("--title", default="Generated Groove", help="song title")
    parser.add_argument("--bpm", type=float, default=100.0, help="tempo in BPM")
    parser.add_argument("--difficulty", type=int, default=1, help="difficulty 1-4")
    parser.add_argument("--length", type=int, default=20, help="length in seconds")
    parser.add_argument("--strings", type=int, default=6, choices=[4, 6], help="string count")
    parser.add_argument("--seed", default=None, help="random seed")

    args = parser.parse_args()

    notes = generate_groove(
        bpm=args.bpm,
        difficulty=args.difficulty,
        length_sec=max(5, args.length),
        string_count=args.strings,
        seed=args.seed,
    )

    payload = {
        "title": args.title,
        "bpm": args.bpm,
        "sync_offset_ms": 0,
        "string_count": args.strings,
        "notes": [note.__dict__ for note in notes],
    }

    with open(args.out, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2)

    print(f"Wrote {len(notes)} notes to {args.out}")


if __name__ == "__main__":
    main()
