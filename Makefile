.PHONY: run face ear deps

SONG ?= assets/sample_song.json
PYTHON := $(shell command -v python3.12 || command -v python3.11 || command -v python3.10 || command -v python3.9 || command -v python3.8 || command -v python3)

run: deps
	./scripts/run.sh "$(SONG)"

face: deps
	go run ./cmd/face "$(SONG)"

ear: deps
	$(PYTHON) ./python/ear.py

deps:
	$(PYTHON) -m venv .venv
	. .venv/bin/activate && pip install --upgrade pip && pip install -r python/requirements.txt
