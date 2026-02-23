import json
import math
import os
import random
import socket
import time

import aubio
import numpy as np
import pyaudio

SOCKET_PATH = os.environ.get("SOCKET_PATH", "/tmp/guitar_ear.sock")
CONNECT_RETRY_SEC = int(os.environ.get("EAR_CONNECT_RETRY_SEC", "10"))
BUFFER_SIZE = 512
WINDOW_SIZE = 2048
SAMPLE_RATE = 44100
CONFIDENCE_MIN = 0.80
FAKE_MODE = os.environ.get("EAR_FAKE", "") == "1"
FAKE_FREQ = float(os.environ.get("EAR_FREQ", "110.0"))
FAKE_INTERVAL_MS = int(os.environ.get("EAR_INTERVAL_MS", "50"))
AI_MODE = os.environ.get("EAR_AI", "") == "1"
AI_SPEED = float(os.environ.get("AI_SPEED", "100"))
AI_DIFFICULTY = int(os.environ.get("AI_DIFFICULTY", "1"))
AI_LENGTH_SEC = int(os.environ.get("AI_LENGTH_SEC", "20"))
AI_SEED = os.environ.get("AI_SEED", "")
AI_LOOP = os.environ.get("AI_LOOP", "1") == "1"
AI_SONG_OUT = os.environ.get("AI_SONG_OUT", "")
EAR_STRINGS = int(os.environ.get("EAR_STRINGS", "6"))
EAR_DEVICE_INDEX = os.environ.get("EAR_DEVICE_INDEX", "")
EAR_DEVICE_NAME = os.environ.get("EAR_DEVICE_NAME", "")
EAR_LIST_DEVICES = os.environ.get("EAR_LIST_DEVICES", "") == "1"


def main() -> None:
    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    start_time = time.time()
    while True:
        try:
            client.connect(SOCKET_PATH)
            break
        except FileNotFoundError:
            pass
        except ConnectionRefusedError:
            pass

        if CONNECT_RETRY_SEC <= 0:
            time.sleep(0.1)
            continue

        if time.time() - start_time > CONNECT_RETRY_SEC:
            raise RuntimeError(
                f"Timed out after {CONNECT_RETRY_SEC}s waiting for socket {SOCKET_PATH}"
            )
        time.sleep(0.1)

    if AI_MODE:
        audio = None
        stream = None
        pitch = None
    elif FAKE_MODE:
        audio = None
        stream = None
        pitch = None
    else:
        audio = pyaudio.PyAudio()
        if EAR_LIST_DEVICES:
            list_input_devices(audio)
            return

        input_device_index = resolve_input_device(audio, EAR_DEVICE_INDEX, EAR_DEVICE_NAME)
        stream = audio.open(
            format=pyaudio.paFloat32,
            channels=1,
            rate=SAMPLE_RATE,
            input=True,
            input_device_index=input_device_index,
            frames_per_buffer=BUFFER_SIZE,
        )

        pitch = aubio.pitch("default", WINDOW_SIZE, BUFFER_SIZE, SAMPLE_RATE)
        pitch.set_unit("Hz")
        pitch.set_silence(-40)

    try:
        if AI_MODE:
            run_ai_mode(client)
            return

        while True:
            if FAKE_MODE:
                payload = json.dumps({"freq": FAKE_FREQ, "conf": 0.99, "ts_ms": int(time.time() * 1000)})
                client.sendall((payload + "\n").encode("utf-8"))
                time.sleep(FAKE_INTERVAL_MS / 1000.0)
                continue

            data = stream.read(BUFFER_SIZE, exception_on_overflow=False)
            samples = np.frombuffer(data, dtype=np.float32)
            freq = float(pitch(samples)[0])
            conf = float(pitch.get_confidence())

            if conf >= CONFIDENCE_MIN and freq > 0:
                payload = json.dumps({"freq": freq, "conf": conf, "ts_ms": int(time.time() * 1000)})
                client.sendall((payload + "\n").encode("utf-8"))
    except KeyboardInterrupt:
        pass
    finally:
        if stream is not None:
            stream.stop_stream()
            stream.close()
        if audio is not None:
            audio.terminate()
        client.close()


def run_ai_mode(client: socket.socket) -> None:
    rng = random.Random()
    if AI_SEED:
        try:
            rng.seed(int(AI_SEED))
        except ValueError:
            rng.seed(AI_SEED)

    groove = generate_groove(
        bpm=AI_SPEED,
        difficulty=max(1, AI_DIFFICULTY),
        length_sec=max(5, AI_LENGTH_SEC),
        string_count=4 if EAR_STRINGS == 4 else 6,
        rng=rng,
    )

    if AI_SONG_OUT:
        write_song(AI_SONG_OUT, groove)

    while True:
        start = time.time()
        for note in groove:
            target_time = start + (note["time_ms"] / 1000.0)
            wait = target_time - time.time()
            if wait > 0:
                time.sleep(wait)
            payload = json.dumps(
                {"freq": note["freq_hz"], "conf": 0.99, "ts_ms": int(time.time() * 1000)}
            )
            client.sendall((payload + "\n").encode("utf-8"))

        if not AI_LOOP:
            break


def generate_groove(
    bpm: float, difficulty: int, length_sec: int, string_count: int, rng: random.Random
) -> list[dict]:
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

    notes: list[dict] = []
    for step in range(total_steps):
        if rng.random() > play_prob:
            continue
        string = rng.randint(1, string_count)
        fret = rng.randint(0, fret_max)
        freq = base_freqs[string - 1] * math.pow(2.0, fret / 12.0)
        notes.append(
            {
                "time_ms": int(step * step_ms),
                "string": string,
                "fret": fret,
                "freq_hz": round(freq, 2),
                "duration_ms": int(step_ms),
            }
        )

    if not notes:
        notes.append(
            {
                "time_ms": 0,
                "string": 1,
                "fret": 0,
                "freq_hz": round(base_freqs[0], 2),
                "duration_ms": int(step_ms),
            }
        )

    return notes


def base_frequencies(string_count: int) -> list[float]:
    if string_count == 4:
        return [98.0, 73.42, 55.0, 41.2]  # G2 D2 A1 E1
    return [329.63, 246.94, 196.0, 146.83, 110.0, 82.41]  # E4 B3 G3 D3 A2 E2


def list_input_devices(audio: pyaudio.PyAudio) -> None:
    info = audio.get_host_api_info_by_index(0)
    count = int(info.get("deviceCount", 0))
    for idx in range(count):
        dev = audio.get_device_info_by_host_api_device_index(0, idx)
        if int(dev.get("maxInputChannels", 0)) <= 0:
            continue
        name = dev.get("name", "")
        rate = dev.get("defaultSampleRate", "")
        print(f"[{idx}] {name} (rate={rate})")


def resolve_input_device(
    audio: pyaudio.PyAudio, device_index: str, device_name: str
) -> int | None:
    if device_index:
        try:
            return int(device_index)
        except ValueError:
            raise ValueError("EAR_DEVICE_INDEX must be an integer")
    if not device_name:
        return None

    info = audio.get_host_api_info_by_index(0)
    count = int(info.get("deviceCount", 0))
    device_name = device_name.lower()
    for idx in range(count):
        dev = audio.get_device_info_by_host_api_device_index(0, idx)
        if int(dev.get("maxInputChannels", 0)) <= 0:
            continue
        name = str(dev.get("name", "")).lower()
        if device_name in name:
            return idx
    raise ValueError(f"EAR_DEVICE_NAME not found: {device_name}")


def write_song(path: str, notes: list[dict]) -> None:
    title = "AI Groove"
    payload = {
        "title": title,
        "bpm": AI_SPEED,
        "sync_offset_ms": 0,
        "string_count": 4 if EAR_STRINGS == 4 else 6,
        "notes": notes,
    }
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2)


if __name__ == "__main__":
    main()
