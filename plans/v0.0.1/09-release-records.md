# v0.0.1 - Release records

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** tag 0.0.1 recorded on 2026-09-24
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

- [ ] `make lint` output pasted below after phase 05 closure.
- [ ] `make test` output pasted below after phase 05 closure.

### 9.3 Tag 0.0.3

- [ ] `make lint` output pasted below after phase 06 closure.
- [ ] `make test` output pasted below after phase 06 closure.

### 9.4 Tag 0.0.4

- [ ] `make lint` output pasted below after phase 07 closure.
- [ ] `make test` output pasted below after phase 07 closure.

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

## Dependencies

The phase file named in each row.
