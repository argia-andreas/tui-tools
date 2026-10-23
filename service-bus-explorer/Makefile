GO ?= go
PYTHON ?= .venv/bin/python

.PHONY: build run-fixture test terminal-test
build:
	$(GO) build -o bin/sbx ./cmd/sbx

run-fixture: build
	./bin/sbx --fixture

test:
	$(GO) test -race ./...

terminal-test: build
	$(PYTHON) scripts/terminal-smoke.py
