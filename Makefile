BIN := bin/spectreps
PKG := ./cmd/spectreps
# -p is how many test binaries run at once. Default is GOMAXPROCS.
# nproc is the machine's CPU count. Fall back to 1 if the command is missing.
NPROC := $(shell nproc 2>/dev/null || echo 1)

.PHONY: help build test lint fmt tidy clean size-check pdfa-check pdfua2-check

help:
	@printf '%s\n' \
		'build       compile $(BIN)' \
		'test        go test -p $(NPROC) ./...' \
		'lint        gofmt check, golangci-lint, and size-check' \
		'size-check  Go files over 2000 lines must be allowlisted' \
		'pdfa-check  run veraPDF over sampledata/pdfa when installed' \
		'pdfua2-check run veraPDF over sampledata/pdfua2 when installed' \
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

# pdfa-check runs veraPDF as a proof tool over the sampled PDF/A writes.
# It is not a dependency and it skips when the CLI is absent. veraPDF is Java,
# so it stays out of make test.
pdfa-check:
	@if ! command -v verapdf >/dev/null 2>&1; then \
		printf '%s\n' 'pdfa-check: verapdf not installed, skipping'; \
		exit 0; \
	fi; \
	files=$$(find sampledata/pdfa -name '*.pdf' 2>/dev/null); \
	if [ -z "$$files" ]; then \
		printf '%s\n' 'pdfa-check: no samples under sampledata/pdfa, skipping'; \
		exit 0; \
	fi; \
	verapdf --flavour 4 $$files

# pdfua2-check runs veraPDF as a proof tool over the sampled PDF/UA-2 writes.
# It is not a dependency and it skips when the CLI is absent. veraPDF is Java,
# so it stays out of make test.
pdfua2-check:
	@if ! command -v verapdf >/dev/null 2>&1; then \
		printf '%s\n' 'pdfua2-check: verapdf not installed, skipping'; \
		exit 0; \
	fi; \
	files=$$(find sampledata/pdfua2 -name '*.pdf' 2>/dev/null); \
	if [ -z "$$files" ]; then \
		printf '%s\n' 'pdfua2-check: no samples under sampledata/pdfua2, skipping'; \
		exit 0; \
	fi; \
	verapdf --flavour ua2 --format json $$files

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin
