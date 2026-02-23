import json
import os
import socket
import time

import aubio
import numpy as np
import pyaudio

SOCKET_PATH = "/tmp/guitar_ear.sock"
BUFFER_SIZE = 512
WINDOW_SIZE = 2048
SAMPLE_RATE = 44100
CONFIDENCE_MIN = 0.80
FAKE_MODE = os.environ.get("EAR_FAKE", "") == "1"
FAKE_FREQ = float(os.environ.get("EAR_FREQ", "110.0"))
FAKE_INTERVAL_MS = int(os.environ.get("EAR_INTERVAL_MS", "50"))


def main() -> None:
    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    client.connect(SOCKET_PATH)

    if FAKE_MODE:
        audio = None
        stream = None
        pitch = None
    else:
        audio = pyaudio.PyAudio()
        stream = audio.open(
            format=pyaudio.paFloat32,
            channels=1,
            rate=SAMPLE_RATE,
            input=True,
            frames_per_buffer=BUFFER_SIZE,
        )

        pitch = aubio.pitch("default", WINDOW_SIZE, BUFFER_SIZE, SAMPLE_RATE)
        pitch.set_unit("Hz")
        pitch.set_silence(-40)

    try:
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


if __name__ == "__main__":
    main()
