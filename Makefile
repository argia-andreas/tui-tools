.PHONY: build demo run run-lab test test-terminal test-terminal-live check lab-up lab-seed lab-down test-integration

build:
	go build -o bin/lazyftp ./cmd/lazyftp

demo run: build
	./bin/lazyftp --demo

test:
	go test ./...

test-terminal: build
	.venv/bin/python scripts/terminal-smoke.py

test-terminal-live: build
	python3 scripts/lab.py check
	.venv/bin/python scripts/terminal-live.py

run-lab: build
	python3 scripts/lab.py check
	./bin/lazyftp --live --rclone-config .lab/rclone.conf --left "$(CURDIR)/.lab/local" --right lab-ftp:/incoming --blob lab-blob:/files/assets

check:
	@test -z "$$(gofmt -l $$(git ls-files --cached --others --exclude-standard '*.go'))" || { echo 'Run gofmt on the Go source files.'; exit 1; }
	go vet ./...
	go test ./...

lab-up:
	python3 scripts/lab.py up

lab-seed:
	python3 scripts/lab.py seed

lab-down:
	python3 scripts/lab.py down

test-integration:
	python3 scripts/lab.py check
	PATH="$$HOME/.local/bin:$$PATH" RCLONE_CONFIG="$(CURDIR)/.lab/rclone.conf" go test -tags=integration ./internal/storage -count=1
