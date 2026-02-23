import math
import struct
import wave
from dataclasses import dataclass
from typing import Callable

SAMPLE_RATE = 44100
BPM = 140.0
DURATION_SEC = 180
CHANNELS = 2

BEAT_SEC = 60.0 / BPM
BAR_BEATS = 4
BAR_SEC = BEAT_SEC * BAR_BEATS

PCM_OUT = "assets/hair_metal.pcm"
WAV_OUT = "assets/hair_metal.wav"


@dataclass
class Event:
    start_sec: float
    duration_sec: float
    freq_hz: float
    amp: float
    wave: str
    pan: float  # -1 left, 1 right


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


def note_freq(note: str) -> float:
    name = note[:-1]
    octave = int(note[-1])
    midi = 12 * (octave + 1) + NOTE_OFFSETS[name]
    return 440.0 * (2 ** ((midi - 69) / 12))


def clamp(v: float, lo: float, hi: float) -> float:
    return max(lo, min(hi, v))


def saw(phase: float) -> float:
    return 2.0 * (phase - math.floor(phase + 0.5))


def square(phase: float) -> float:
    return 1.0 if (phase % 1.0) < 0.5 else -1.0


def sine(phase: float) -> float:
    return math.sin(2.0 * math.pi * phase)


def noise(_phase: float) -> float:
    return (2.0 * (math.sin(2.0 * math.pi * _phase * 12345.0) % 1.0)) - 1.0


def envelope(t: float, duration: float, attack: float = 0.01, decay: float = 0.1) -> float:
    if t < 0:
        return 0.0
    if t < attack:
        return t / attack
    if t > duration:
        return 0.0
    tail = duration - t
    if tail < decay:
        return tail / decay
    return 1.0


def render_events(events: list[Event], total_samples: int) -> list[float]:
    samples = [0.0] * total_samples
    for ev in events:
        start_idx = int(ev.start_sec * SAMPLE_RATE)
        end_idx = min(total_samples, int((ev.start_sec + ev.duration_sec) * SAMPLE_RATE))
        if end_idx <= start_idx:
            continue
        if ev.wave == "saw":
            wave_fn: Callable[[float], float] = saw
        elif ev.wave == "square":
            wave_fn = square
        elif ev.wave == "noise":
            wave_fn = noise
        else:
            wave_fn = sine

        for i in range(start_idx, end_idx):
            t = (i - start_idx) / SAMPLE_RATE
            env = envelope(t, ev.duration_sec)
            phase = (t * ev.freq_hz) % 1.0 if ev.freq_hz > 0 else 0.0
            samples[i] += wave_fn(phase) * ev.amp * env

    return samples


def render_kick(start_sec: float) -> list[Event]:
    return [Event(start_sec, 0.12, 90.0, 0.9, "sine", 0.0)]


def render_snare(start_sec: float) -> list[Event]:
    return [
        Event(start_sec, 0.10, 180.0, 0.4, "sine", 0.0),
        Event(start_sec, 0.08, 400.0, 0.5, "noise", 0.0),
    ]


def render_hat(start_sec: float) -> list[Event]:
    return [Event(start_sec, 0.03, 8000.0, 0.15, "noise", 0.0)]


def section_for_bar(bar: int) -> str:
    if bar < 8:
        return "intro"
    if bar < 24:
        return "verse"
    if bar < 32:
        return "pre"
    if bar < 40:
        return "chorus"
    if bar < 56:
        return "verse"
    if bar < 64:
        return "pre"
    if bar < 72:
        return "chorus"
    if bar < 80:
        return "guitar_solo"
    if bar < 84:
        return "bass_solo"
    if bar < 100:
        return "final_chorus"
    return "outro"


def chord_root_for_bar(bar: int) -> str:
    progression = ["E", "G", "A", "C"]
    return progression[bar % len(progression)]


def build_events() -> tuple[list[Event], list[Event]]:
    guitar_events: list[Event] = []
    bass_events: list[Event] = []
    drum_events: list[Event] = []
    lead_events: list[Event] = []

    total_bars = int(DURATION_SEC / BAR_SEC)

    for bar in range(total_bars):
        section = section_for_bar(bar)
        bar_start = bar * BAR_SEC
        root = chord_root_for_bar(bar)

        # Drums: kick on 1/3, snare on 2/4, hats on 8ths.
        drum_events += render_kick(bar_start)
        drum_events += render_kick(bar_start + 2 * BEAT_SEC)
        drum_events += render_snare(bar_start + 1 * BEAT_SEC)
        drum_events += render_snare(bar_start + 3 * BEAT_SEC)
        for h in range(8):
            drum_events += render_hat(bar_start + h * (BEAT_SEC / 2))

        # Bassline: 8th notes on root and fifth.
        root_note = f"{root}2" if section != "bass_solo" else f"{root}1"
        fifth_note = {
            "E": "B2",
            "G": "D3",
            "A": "E3",
            "C": "G3",
        }[root]
        for h in range(8):
            note = root_note if h % 2 == 0 else fifth_note
            freq = note_freq(note)
            amp = 0.35 if section != "bass_solo" else 0.55
            bass_events.append(Event(bar_start + h * (BEAT_SEC / 2), BEAT_SEC / 2, freq, amp, "sine", -0.2))

        # Guitar power chords on beats 1 and 3.
        guitar_root = f"{root}3"
        guitar_fifth = {
            "E": "B3",
            "G": "D4",
            "A": "E4",
            "C": "G4",
        }[root]
        for beat in (0, 2):
            start = bar_start + beat * BEAT_SEC
            amp = 0.35 if section not in ("guitar_solo", "bass_solo") else 0.2
            guitar_events.append(Event(start, BEAT_SEC * 1.5, note_freq(guitar_root), amp, "saw", 0.2))
            guitar_events.append(Event(start, BEAT_SEC * 1.5, note_freq(guitar_fifth), amp, "saw", 0.2))

        # Lead vocals: only in chorus sections.
        if section in ("chorus", "final_chorus"):
            melody = ["E4", "G4", "A4", "B4", "D5", "B4", "A4", "G4"]
            for i, name in enumerate(melody):
                start = bar_start + i * (BEAT_SEC / 2)
                lead_events.append(Event(start, BEAT_SEC / 2, note_freq(name), 0.22, "square", 0.0))

        # Guitar solo: fast notes in E minor.
        if section == "guitar_solo":
            solo_notes = ["E4", "G4", "A4", "B4", "D5", "E5", "D5", "B4"]
            for i, name in enumerate(solo_notes * 2):
                start = bar_start + i * (BEAT_SEC / 4)
                lead_events.append(Event(start, BEAT_SEC / 4, note_freq(name), 0.3, "saw", 0.2))

        # Bass solo: extra notes.
        if section == "bass_solo":
            solo_notes = ["E2", "G2", "A2", "C3", "B2", "A2", "G2", "E2"]
            for i, name in enumerate(solo_notes * 2):
                start = bar_start + i * (BEAT_SEC / 4)
                bass_events.append(Event(start, BEAT_SEC / 4, note_freq(name), 0.6, "sine", -0.2))

    return guitar_events + lead_events + drum_events, bass_events


def mix_to_pcm() -> None:
    total_samples = int(DURATION_SEC * SAMPLE_RATE)

    main_events, bass_events = build_events()
    main_track = render_events(main_events, total_samples)
    bass_track = render_events(bass_events, total_samples)

    pcm_frames = bytearray()
    for i in range(total_samples):
        left = main_track[i] + bass_track[i] * 0.9
        right = main_track[i] + bass_track[i] * 0.7
        left = clamp(left, -1.0, 1.0)
        right = clamp(right, -1.0, 1.0)
        pcm_frames += struct.pack("<hh", int(left * 32767), int(right * 32767))

    with open(PCM_OUT, "wb") as handle:
        handle.write(pcm_frames)

    with wave.open(WAV_OUT, "wb") as wav:
        wav.setnchannels(CHANNELS)
        wav.setsampwidth(2)
        wav.setframerate(SAMPLE_RATE)
        wav.writeframes(pcm_frames)

    print(f"Wrote {PCM_OUT} and {WAV_OUT}")


if __name__ == "__main__":
    mix_to_pcm()
