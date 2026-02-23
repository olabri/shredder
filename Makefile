.PHONY: run face ear deps headless ascii

SONG ?= assets/sample_song.json
PYTHON := $(shell command -v python3.11 || command -v python3.12 || command -v python3.10 || command -v python3.9 || command -v python3.8 || command -v python3)

run: deps
	./scripts/run.sh "$(SONG)"

face: deps
	go run ./cmd/face "$(SONG)"

ear: deps
	$(PYTHON) ./python/ear.py

headless: deps
	HEADLESS=1 ./scripts/run.sh "$(SONG)"

ascii: deps
	ASCII=1 ./scripts/run.sh "$(SONG)"

deps:
	$(PYTHON) -m venv .venv
	. .venv/bin/activate && pip install --upgrade pip setuptools wheel && pip install "numpy<2.0" && pip install pyaudio && CFLAGS="-Wno-incompatible-function-pointer-types" pip install --no-build-isolation "aubio==0.4.9"
