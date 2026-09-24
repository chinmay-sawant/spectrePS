# v0.0.1 - Release records

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** tag 0.0.4 recorded on 2026-09-25
> **Estimated effort:** part of each tag, not a separate build

---

## Overview

Paste the tag-time `make lint` and `make test` transcripts under the matching heading. A dev-loop run does not fill these rows. If a later rerun disagrees with the pasted transcript, add a new line with the date. Do not delete the old line.

## Executive summary

The phase files own the behavior rows. This file owns the proof that the tag was cut on a green lint and test.

## Phase 9: Release records

### 9.1 Tag 0.0.1

- [x] `make lint` output pasted below after phase 02 closure. Outcome on 2026-09-24: exit 0.
- [x] `make test` output pasted below after phase 02 closure. Outcome on 2026-09-24: exit 0.

### 9.2 Tag 0.0.2

- [x] `make lint` output pasted below after phase 05 closure. Outcome on 2026-09-24: exit 0.
- [x] `make test` output pasted below after phase 05 closure. Outcome on 2026-09-24: exit 0.

### 9.3 Tag 0.0.3

- [x] `make lint` output pasted below after phase 06 closure. Outcome on 2026-09-24: exit 0.
- [x] `make test` output pasted below after phase 06 closure. Outcome on 2026-09-24: exit 0.

### 9.4 Tag 0.0.4

- [x] `make lint` output pasted below after phase 07 closure. Outcome on 2026-09-25: exit 0.
- [x] `make test` output pasted below after phase 07 closure. Outcome on 2026-09-25: exit 0.

### 9.5 Tag 0.0.5

- [ ] `make lint` output pasted below after phase 08 closure.
- [ ] `make test` output pasted below after phase 08 closure.

## Transcripts

### Tag 0.0.1, 2026-09-24

`make lint`

```
go vet ./...
```

Exit 0. `gofmt -l .` printed nothing before `go vet`.

`make test`

```
go test ./...
ok  	github.com/chinmay-sawant/spectrePS	0.004s
ok  	github.com/chinmay-sawant/spectrePS/cmd/spectreps	0.017s
```

Exit 0.

### Layout, 2026-09-24

The public library moved to `spectreps/`. The command entry stayed `cmd/spectreps`. Session, compare, and flag handling moved under `internal/`.

`make lint`

```
go vet ./...
```

Exit 0. `gofmt -l .` printed nothing before `go vet`.

`make test`

```
go test ./...
?   	github.com/chinmay-sawant/spectrePS/cmd/spectreps	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/cli	0.008s
?   	github.com/chinmay-sawant/spectrePS/internal/engine	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.002s
```

Exit 0.

### Lint target, 2026-09-24

`make lint` now runs `gofmt -l` and then `golangci-lint run ./...`.

`make lint`

```
golangci-lint run ./...
```

Exit 0. `gofmt -l .` printed nothing before golangci-lint.

`make test`

```
go test ./...
?   	github.com/chinmay-sawant/spectrePS/cmd/spectreps	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/cli	0.009s
?   	github.com/chinmay-sawant/spectrePS/internal/engine	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.002s
```

Exit 0.

### Tag 0.0.2, 2026-09-24

`make lint`

```
golangci-lint run ./...
```

Exit 0. `gofmt -l .` printed nothing before golangci-lint.

`make test`

```
go test ./...
?   	github.com/chinmay-sawant/spectrePS/cmd/spectreps	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/cli	0.016s
?   	github.com/chinmay-sawant/spectrePS/internal/engine	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/graphics	0.002s
ok  	github.com/chinmay-sawant/spectrePS/internal/ps	0.007s
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.007s
```

Exit 0.

### Tag 0.0.3, 2026-09-24

`make lint`

```
golangci-lint run ./...
```

Exit 0. `gofmt -l .` printed nothing before golangci-lint.

`make test`

```
go test ./...
?   	github.com/chinmay-sawant/spectrePS/cmd/spectreps	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/cli	0.034s
?   	github.com/chinmay-sawant/spectrePS/internal/engine	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/graphics	0.005s
ok  	github.com/chinmay-sawant/spectrePS/internal/pdf	0.011s
ok  	github.com/chinmay-sawant/spectrePS/internal/ps	0.017s
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.017s
```

Exit 0.

### Tag 0.0.4, 2026-09-25

`make lint`

```
golangci-lint run ./...
make size-check
bash scripts/check-file-size.sh
size-check: clean (0 over-limit files).
```

Exit 0. `gofmt -l .` printed nothing before golangci-lint.

`make test`

```
go test ./...
?   	github.com/chinmay-sawant/spectrePS/cmd/spectreps	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/cli	0.032s
?   	github.com/chinmay-sawant/spectrePS/internal/engine	[no test files]
ok  	github.com/chinmay-sawant/spectrePS/internal/graphics	0.003s
ok  	github.com/chinmay-sawant/spectrePS/internal/pdf	0.011s
ok  	github.com/chinmay-sawant/spectrePS/internal/pdfout	0.012s
ok  	github.com/chinmay-sawant/spectrePS/internal/ps	0.016s
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.027s
```

Exit 0.

## Dependencies

The phase file named in each row.
