BIN := bin/spectreps
PKG := ./cmd/spectreps
# -p is how many test binaries run at once. Default is GOMAXPROCS.
# nproc is the machine's CPU count. Fall back to 1 if the command is missing.
NPROC := $(shell nproc 2>/dev/null || echo 1)

.PHONY: help build test lint fmt tidy clean size-check

help:
	@printf '%s\n' \
		'build       compile $(BIN)' \
		'test        go test -p $(NPROC) ./...' \
		'lint        gofmt check, golangci-lint, and size-check' \
		'size-check  Go files over 2000 lines must be allowlisted' \
		'fmt         gofmt -w .' \
		'tidy        go mod tidy' \
		'clean       remove bin/'

build:
	go build -trimpath -o $(BIN) $(PKG)

test:
	go test -p $(NPROC) ./...

lint:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		printf '%s\n' "gofmt needed:" $$files; \
		exit 1; \
	fi
	golangci-lint run ./...
	$(MAKE) size-check

# Go files over 2,000 lines must match scripts/file-size-allowlist.txt.
# The rule is AGENTS.md, Code structure. lint runs this target.
size-check:
	bash scripts/check-file-size.sh

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin
