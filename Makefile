.PHONY: build demo run test test-terminal check

build:
	go build -o bin/lazyftp ./cmd/lazyftp

demo run: build
	./bin/lazyftp --demo

test:
	go test ./...

test-terminal: build
	.venv/bin/python scripts/terminal-smoke.py

check:
	@test -z "$$(gofmt -l $$(git ls-files --cached --others --exclude-standard '*.go'))" || { echo 'Run gofmt on the Go source files.'; exit 1; }
	go vet ./...
	go test ./...
