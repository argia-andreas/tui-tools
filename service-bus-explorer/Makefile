GO ?= go
PYTHON ?= .venv/bin/python
LAB_FLAGS ?=
LAB_ARGS ?=

.PHONY: build run-fixture run-emulator test terminal-test integration-test lab-up lab-status lab-down lab-reset lab-seed lab-send lab-peek lab-dead-letter lab-drain
build:
	$(GO) build -o bin/sbx ./cmd/sbx

run-fixture: build
	./bin/sbx --fixture

run-emulator: build
	./bin/sbx --emulator

test:
	$(GO) test -race ./...
	python3 -m unittest discover -s scripts -p lab_test.py

terminal-test: build
	$(PYTHON) scripts/terminal-smoke.py

# Seed first; these checks never reset or silently switch to fixture data.
integration-test: build
	$(GO) test -tags integration -count=1 -timeout 120s -v ./internal/azure
	$(PYTHON) scripts/terminal-smoke.py --emulator

lab-up lab-status lab-down lab-reset lab-seed lab-send lab-peek lab-dead-letter lab-drain:
	PATH="$(dir $(GO)):$$PATH" python3 scripts/lab.py $(LAB_FLAGS) $(@:lab-%=%) $(LAB_ARGS)
