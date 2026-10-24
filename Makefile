.PHONY: build demo run run-lab test test-terminal test-terminal-live test-terminal-panes check lab-up lab-seed lab-check lab-down test-integration

# Open S3 with: make run-lab LAB_RIGHT=lab-s3:/files/assets
LAB_RIGHT ?= lab-ftp:/incoming

build:
	go build -o bin/lazyftp ./cmd/lazyftp
	go build -o bin/lazyftp-mcp ./cmd/lazyftp-mcp

demo: build
	./bin/lazyftp --demo

run: build
	./bin/lazyftp

test:
	go test ./...

test-terminal: build
	.venv/bin/python scripts/terminal-smoke.py

test-terminal-live: build
	python3 scripts/lab.py check
	.venv/bin/python scripts/terminal-live.py
	.venv/bin/python scripts/terminal-rename.py

test-terminal-panes: build
	python3 scripts/lab.py check
	.venv/bin/python scripts/terminal-panes.py

run-lab: build
	python3 scripts/lab.py check
	./bin/lazyftp --live --rclone-config .lab/rclone.conf --left "$(CURDIR)/.lab/local" --right "$(LAB_RIGHT)" --remote lab-ftp:/incoming --remote lab-blob:/files/assets --remote lab-s3:/files/assets

check:
	@test -z "$$(gofmt -l $$(git ls-files --cached --others --exclude-standard '*.go'))" || { echo 'Run gofmt on the Go source files.'; exit 1; }
	go vet ./...
	go test ./...

lab-up:
	python3 scripts/lab.py up

lab-seed:
	python3 scripts/lab.py seed

lab-check:
	python3 scripts/lab.py check

lab-down:
	python3 scripts/lab.py down

test-integration:
	python3 scripts/lab.py check
	PATH="$$HOME/.local/bin:$$PATH" RCLONE_CONFIG="$(CURDIR)/.lab/rclone.conf" go test -tags=integration ./internal/storage ./internal/ui ./cmd/lazyftp-mcp -count=1
