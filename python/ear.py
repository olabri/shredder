import json
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


def main() -> None:
    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    client.connect(SOCKET_PATH)

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
        stream.stop_stream()
        stream.close()
        audio.terminate()
        client.close()


if __name__ == "__main__":
    main()
