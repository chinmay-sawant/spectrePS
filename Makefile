BIN := bin/spectreps
PKG := ./cmd/spectreps
# -p is how many test binaries run at once. Default is GOMAXPROCS.
# nproc is the machine's CPU count. Fall back to 1 if the command is missing.
NPROC := $(shell nproc 2>/dev/null || echo 1)
# BENCH_PKGS is the packages with Benchmark functions. make test never runs them.
BENCH_PKGS := ./spectreps ./internal/cli ./internal/engine ./internal/font ./internal/pdf ./internal/pdfa ./internal/pdfout ./internal/graphics ./internal/ps ./internal/psout ./internal/tag

.PHONY: help build test lint fmt tidy clean size-check pdfa-check pdfua2-check refs-gs-check validation-run bench bench-profile bench-check

help:
	@printf '%s\n' \
		'build       compile $(BIN)' \
		'test        go test -p $(NPROC) ./...' \
		'lint        gofmt check, golangci-lint, and size-check' \
		'size-check  Go files over 2000 lines must be allowlisted' \
		'pdfa-check  run veraPDF over sampledata/pdfa when installed' \
		'pdfua2-check run veraPDF over sampledata/pdfua2 when installed' \
		'refs-gs-check run the Ghostscript reference proofs when gs is installed' \
		'validation-run run the acceptance harness over sampledata/validation' \
		'bench       run the benchmarks with -count=3 into profiles/bench.txt' \
		'bench-profile write CPU and memory profiles per package under profiles/' \
		'bench-check run the benchmarks with -count=5 and compare with benchstat' \
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
# Base files go through the 4 profile and the 4f- files through 4f. It is not
# a dependency and it skips when the CLI is absent. veraPDF is Java, so it
# stays out of make test.
pdfa-check:
	@verapdf=$$(if [ -x ./verapdf/verapdf ]; then printf '%s' ./verapdf/verapdf; elif command -v verapdf >/dev/null 2>&1; then command -v verapdf; fi); \
	if [ -z "$$verapdf" ]; then \
		printf '%s\n' 'pdfa-check: verapdf not installed, skipping'; \
		exit 0; \
	fi; \
	roots='sampledata/pdfa sampledata/validation/pdfa'; \
	base=$$(find $$roots -name '*.pdf' ! -path '*/negative/*' ! -name '4f-*' 2>/dev/null); \
	foured=$$(find $$roots -name '4f-*.pdf' ! -path '*/negative/*' 2>/dev/null); \
	if [ -z "$$base" ] && [ -z "$$foured" ]; then \
		printf '%s\n' 'pdfa-check: no samples under sampledata/pdfa or sampledata/validation/pdfa, skipping'; \
		exit 0; \
	fi; \
	status=0; \
	if [ -n "$$base" ]; then "$$verapdf" --flavour 4 $$base || status=1; fi; \
	if [ -n "$$foured" ]; then "$$verapdf" --flavour 4f $$foured || status=1; fi; \
	exit $$status

# pdfua2-check runs veraPDF as a proof tool over the sampled PDF/UA-2 writes.
# It is not a dependency and it skips when the CLI is absent. veraPDF is Java,
# so it stays out of make test.
pdfua2-check:
	@verapdf=$$(if [ -x ./verapdf/verapdf ]; then printf '%s' ./verapdf/verapdf; elif command -v verapdf >/dev/null 2>&1; then command -v verapdf; fi); \
	if [ -z "$$verapdf" ]; then \
		printf '%s\n' 'pdfua2-check: verapdf not installed, skipping'; \
		exit 0; \
	fi; \
	files=$$(find sampledata/pdfua2 sampledata/validation/tagged -name '*.pdf' ! -path '*/negative/*' 2>/dev/null); \
	if [ -z "$$files" ]; then \
		printf '%s\n' 'pdfua2-check: no samples under sampledata/pdfua2 or sampledata/validation/tagged, skipping'; \
		exit 0; \
	fi; \
	"$$verapdf" --flavour ua2 --format json $$files

# refs-gs-check runs the phase 11 Ghostscript reference proofs. gs is a proof
# tool only: the target prints a skip when it is absent, and it never runs
# inside make test or make lint.
refs-gs-check:
	bash scripts/ref-gs-check.sh

# validation-run runs the acceptance harness over the validation corpus. It
# never runs under make test or make lint.
validation-run:
	bash scripts/validation-run.sh

# bench runs every benchmark three times and records the output. Benchmark
# timing is machine-specific and never gates make test.
bench:
	@mkdir -p profiles
	go test -p 1 -run '^$$' -bench . -benchmem -count=3 $(BENCH_PKGS) > profiles/bench.txt 2>&1
	@printf '%s\n' 'bench: wrote profiles/bench.txt'

# bench-profile writes one CPU and one memory profile per benchmark package.
bench-profile:
	@mkdir -p profiles
	@for pkg in $(BENCH_PKGS); do \
		name=$$(basename $$pkg); \
		printf '%s\n' "bench-profile: $$name"; \
		go test -p 1 -run '^$$' -bench . -benchmem -count=1 \
			-cpuprofile profiles/$$name.cpu \
			-memprofile profiles/$$name.mem \
			$$pkg > profiles/$$name.profile.txt 2>&1 || exit 1; \
	done
	@printf '%s\n' 'bench-profile: wrote profiles/*.cpu and profiles/*.mem'

# bench-check compares a fresh -count=5 run against profiles/bench.txt with
# benchstat when it is on PATH, and records the verdict in profiles/compare.txt.
bench-check:
	@mkdir -p profiles
	go test -p 1 -run '^$$' -bench . -benchmem -count=5 $(BENCH_PKGS) > profiles/bench-check.txt 2>&1
	@if command -v benchstat >/dev/null 2>&1; then \
		benchstat profiles/bench.txt profiles/bench-check.txt > profiles/compare.txt; \
	else \
		printf '%s\n' 'benchstat is not on PATH, skipping the comparison' > profiles/compare.txt; \
	fi
	@cat profiles/compare.txt

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin
