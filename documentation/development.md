# Development

`go.mod` sets `go 1.26.4`. `go version` on this machine prints `go1.26.4`.

## Make targets

| Target | Command |
| --- | --- |
| `make build` | `go build -trimpath -o bin/spectreps ./cmd/spectreps` |
| `make test` | `go test ./...` |
| `make lint` | `gofmt -l` must be empty, then `golangci-lint run ./...` |
| `make fmt` | `gofmt -w .` |
| `make tidy` | `go mod tidy` |
| `make clean` | remove `bin/` |

`make build` compiles `./cmd/spectreps` to `bin/spectreps`. The public library is `spectreps/`. Private code is `internal/cli` and `internal/engine`. The module root has no `.go` files.

`make lint` and `make test` are the gates for a phase that changes Go code. Record both commands and their outcomes in the phase file before marking that phase complete. A documentation-only change does not run them. The rule comes from `skills/phase-wise-checklist/SKILLS.md`.

## Checklist

`plans/v0.0.1/00-program.md` is the map. Each other file in that directory is the ledger for one phase. Check a row only after the proof in the row has been run on the current tree. `[~]` means deferred, and the only deferred list is `plans/v0.0.1/10-deferred.md`.

When a tag ships, append the `make lint` and `make test` transcript to `plans/v0.0.1/09-release-records.md`. Dev-loop runs and the tag run are both recorded there if they differ. A green dev loop is not a substitute for the tag line.

## Dependencies

The module starts with no third-party requirements. Flate, PNG, and PDF parsing use the standard library. Adding a module requirement is a plan row of its own, with the reason written next to the `go.sum` change.

## Reference install

System Ghostscript at `/usr/bin/gs` is a behavior reference for the docs. Tests do not call it. Fixtures are Spectre output, checked in under `testdata/` when phase 04 adds the first one.
