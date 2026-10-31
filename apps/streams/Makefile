GO ?= go
PYTHON ?= .venv/bin/python

.PHONY: build run-fixture check test test-terminal lab-install lab-up lab-seed lab-check lab-down
build:
	$(GO) build -o bin/streams ./cmd/streams
run-fixture: build
	./bin/streams --fixture
check:
	$(GO) vet ./...
	$(GO) test -race ./...
test:
	$(GO) test -race ./...
test-terminal: build
	$(PYTHON) scripts/terminal-smoke.py
lab-install lab-up lab-seed lab-check lab-down:
	python3 scripts/lab.py $(patsubst lab-%,%,$@)
