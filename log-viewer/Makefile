GO ?= go
PYTHON ?= .venv/bin/python

.PHONY: build run-fixture test check test-terminal
build:
	$(GO) build -o bin/logview ./cmd/logview

run-fixture: build
	./bin/logview --fixture

test:
	$(GO) test -race ./...

check:
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go'))" || { echo 'Run gofmt on Go sources'; exit 1; }
	$(GO) vet ./...
	$(GO) test -race ./...

test-terminal: build
	$(PYTHON) scripts/terminal-smoke.py
