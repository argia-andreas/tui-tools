GO ?= go
MODULES := $(patsubst %/go.mod,%,$(wildcard apps/*/go.mod lib/*/go.mod))

.PHONY: build test vet check format-check test-terminal

# ./build also accepts an app name directly; make build APP=logs is equivalent.
build:
	./build $(APP)

test:
	@set -e; for module in $(MODULES); do GOWORK=off $(GO) -C "$$module" test -race ./...; done
	python3 -m unittest discover -s apps/queue/scripts -p lab_test.py

vet:
	@set -e; for module in $(MODULES); do GOWORK=off $(GO) -C "$$module" vet ./...; done

format-check:
	@test -z "$$(find $(MODULES) -type d \( -name .venv -o -name node_modules -o -name .lab -o -name bin \) -prune -o -type f -name '*.go' -print0 | xargs -0 gofmt -l)" || { echo 'Run gofmt on the Go source files.'; exit 1; }

check: format-check vet test

test-terminal:
	$(MAKE) -C apps/transfer test-terminal
	$(MAKE) -C apps/queue terminal-test
	$(MAKE) -C apps/logs test-terminal
