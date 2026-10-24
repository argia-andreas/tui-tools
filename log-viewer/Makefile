GO ?= go
PYTHON ?= .venv/bin/python

.PHONY: build run-fixture test check test-terminal lab-install lab-up lab-seed lab-check lab-down test-integration run-lab test-terminal-lab
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

lab-install lab-up lab-seed lab-check lab-down:
	python3 scripts/lab.py $(patsubst lab-%,%,$@)

test-integration:
	LOGVIEW_LAB_CONFIG=$(CURDIR)/.lab/source.json $(GO) test -race -count=1 -v ./internal/elastic -run TestLocalLab

run-lab: build
	./bin/logview --config .lab/source.json

test-terminal-lab: build
	$(PYTHON) scripts/terminal-smoke.py --lab
