# Development

`go.mod` sets `go 1.26.4`. `go version` on this machine prints `go1.26.4`.

## Make targets

| Target | Command |
| --- | --- |
| `make build` | `go build -trimpath -o bin/spectreps ./cmd/spectreps` |
| `make test` | `go test -p $(nproc) ./...` |
| `make lint` | `gofmt -l` must be empty, then `golangci-lint run ./...`, then `make size-check` |
| `make size-check` | `bash scripts/check-file-size.sh` |
| `make pdfa-check` | veraPDF over the PDF/A samples, skipped when veraPDF is not installed |
| `make pdfa-corpus-check` | `python3 scripts/check-pdfa-profiles.py` over one compliant sample per PDF/A profile and three negative controls |
| `make pdfua2-check` | veraPDF over the PDF/UA-2 samples, skipped when veraPDF is not installed |
| `make refs-gs-check` | `bash scripts/ref-gs-check.sh`, the phase 11 Ghostscript reference proofs |
| `make validation-run` | `bash scripts/validation-run.sh`, the acceptance harness over the manifest corpus |
| `make validation-fetch` | `go run internal/validation/gen.go -fetch-external`, populates the gitignored live tier; `BULK=1` adds the batch2 bulk tier, a 4.5 GB download |
| `make validation-verify` | `go run internal/validation/gen.go -fetch-external -verify-only`, checks the cache with no network; `BULK=1` verifies the extracted bulk files too |
| `make validation-report` | `bash scripts/validation-report.sh`, the per-area and per-basis pass rate into `profiles/` |
| `make bench` | `go test -p 1 -run '^$' -bench . -benchmem -count=3` over the eleven benchmark packages, writing `profiles/bench.txt` |
| `make bench-profile` | the same packages with `-cpuprofile` and `-memprofile`, writing one CPU and one heap profile per package under `profiles/` |
| `make bench-check` | a `-count=5` rerun compared against `profiles/bench.txt` with benchstat, writing `profiles/compare.txt` |
| `make fmt` | `gofmt -w .` |
| `make tidy` | `go mod tidy` |
| `make clean` | remove `bin/` |

`make build` compiles `./cmd/spectreps` to `bin/spectreps`. The public library is `spectreps/`. Private code is `internal/cli`, `internal/engine`, `internal/ps`, `internal/graphics`, `internal/pdf`, `internal/pdfout`, `internal/pdfa`, `internal/font`, and `internal/psout`. The module root has no `.go` files.

`make lint` and `make test` are the gates for a phase that changes Go code. Record both commands and their outcomes in the phase file before marking that phase complete. A documentation-only change does not run them. The rule comes from `skills/phase-wise-checklist/SKILLS.md`.

`make test` runs the fetched rows of the validation corpus when `sampledata/validation/external/` is present and skips them when it is absent, so a fresh clone and CI stay green without a network. To run the same rows a provisioned machine runs, populate the tier once:

```sh
make validation-fetch
make validation-verify
```

`make validation-fetch` runs `go run internal/validation/gen.go -fetch-external` on every machine. It downloads each external row named in `sampledata/validation/manifest.tsv`, checks the SHA-256 and byte count, and hardlinks the file from a content-addressed cache, so a second run costs no network. `make validation-verify` checks the cache against the manifest without touching the network. Neither target runs under `make test` or `make lint`. `documentation/test.md` and `sampledata/validation/README.md` carry the corpus contract.

`sampledata/validation/bulk.tsv` pins the bulk tier, which is whole tarballs instead of individual files. Its only row is batch2 from the PDF Association stressful corpus: 5,613 files and a 4.5 GB download. The tier is opt-in because of the size. `make validation-fetch BULK=1` checks the archive SHA-512, extracts to a staging directory, checks every member against `bulk/batch2.members.tsv`, and only then moves files into the tree. A plain `make validation-fetch` prints the cost and downloads nothing.

`make size-check` enforces the 2,000-line Go file limit from `AGENTS.md`. Overflow is recorded in `scripts/file-size-allowlist.txt`. `make lint` runs the check, so a stale record fails lint.

`make bench`, `make bench-profile`, and `make bench-check` are manual targets. Neither `make test` nor `make lint` calls them, because `go test ./...` runs benchmarks only when `-bench` is passed. Benchmark output lands in `profiles/`, which is gitignored; the recorded tables live in `documentation/performance.md` and the full run output lives in `documentation/benchmark.md`. `bash scripts/bench-cli.sh` measures the built binary and writes `profiles/cli.txt`. `make bench-check` needs `benchstat` on `PATH`; install it with `go install golang.org/x/perf/cmd/benchstat@latest`, which leaves `go.mod` and `go.sum` untouched.

## Checklist

`plans/v0.0.1/00-program.md`, `plans/v0.0.2/00-program.md`, `plans/v0.0.3/00-program.md`, and `plans/v0.0.4/00-program.md` are the maps. Each other file in those directories is the ledger for one phase. Check a row only after the proof in the row has been run on the current tree. `[~]` means deferred, and the deferred lists are `plans/v0.0.1/10-deferred.md` with `plans/v0.0.3/10-pdfua2.md` for the tag-level rows.

When a tag ships, append the `make lint` and `make test` transcript to `plans/v0.0.1/09-release-records.md`. Dev-loop runs and the tag run are both recorded there if they differ. A green dev loop is not a substitute for the tag line.

## Dependencies

The module started with no third-party requirements. `golang.org/x/image v0.46.0` was the first: it supplies `tiff.Encode` for raster `.tif` and `.tiff` output, `ccitt` for CCITT image streams, and `font/sfnt` and `vector` for text. v0.0.3 added `github.com/mrjoshuak/go-jpeg2000 v1.5.12` for JPEG2000 decode, which pulls `golang.org/x/sys` and `golang.org/x/text` indirectly. Flate, PNG, JPEG, and PDF parsing use the standard library. Adding a module requirement is a plan row of its own, with the reason written next to the requirement in `go.mod`, and `go mod tidy` writes `go.sum`.

## Reference install

System Ghostscript at `/usr/bin/gs` is a behavior reference for the docs. Tests do not call it. Fixtures are Spectre output, checked in under `sampledata/fixtures/`.
