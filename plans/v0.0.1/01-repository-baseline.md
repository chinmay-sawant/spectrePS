# v0.0.1 - Repository baseline

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** written in the planning change, proofs below
> **Estimated effort:** done as documentation

---

## Overview

This phase is the module identity, the make targets, the contracts, and the git remote. It does not add a Go package beyond `go.mod`. `make build` is expected to fail until phase 02 adds `cmd/spectreps`.

## Executive summary

The module path is `github.com/chinmay-sawant/spectrePS`. The `go` line is `1.26.4`. Docs under `documentations/` are the behavior source. Plans under `plans/v0.0.1/` are the ledger. `skills/unslop/SKILL.md` is the local copy of the writing skill.

## Phase 1: Repository baseline

### 1.1 Module

- [x] `go.mod` declares `module github.com/chinmay-sawant/spectrePS` and `go 1.26.4`. Proof: `go list -m` prints `github.com/chinmay-sawant/spectrePS`. `go version` prints `go1.26.4`.

### 1.2 Make

- [x] `Makefile` defines `help`, `build`, `test`, `lint`, `fmt`, `tidy`, and `clean`. `lint` is `gofmt -l` plus `go vet ./...`. `build` outputs `bin/spectreps` from `./cmd/spectreps`.

### 1.3 Docs

- [x] `documentations/README.md` indexes the contracts.
- [x] `documentations/ghostscript-baseline.md` records local `gs` 9.55.0 and the 10.08 manual split between raster, rewrite, validate, and compare.
- [x] `documentations/architecture.md` records the instance, the two front ends, and the device seam.
- [x] `documentations/folder-structure.md` records the tree and the import rule.
- [x] `documentations/public-api.md` records exported signatures.
- [x] `documentations/cli.md` records subcommands and exit codes.
- [x] `documentations/language.md` records the PostScript subset, errors, limits, and banned operators.
- [x] `documentations/devices.md` records pixmap layout, PPM, PNG, compare, rewrite, and the PDF operator subset.
- [x] `documentations/development.md` records make targets and the checklist rule.
- [x] `AGENTS.md` points at those files and at `skills/unslop/SKILL.md`.
- [x] `README.md` names the command, the import path, and this ledger.

### 1.4 Skills and ignore file

- [x] `skills/unslop/SKILL.md` is present in the repo.
- [x] `.gitignore` ignores `bin/`, test binaries, coverage output, and `vendor/`.

### 1.5 Git remote

- [x] Branch `master`, origin `https://github.com/chinmay-sawant/spectrePS.git`. Proof: `git remote get-url origin` prints `https://github.com/chinmay-sawant/spectrePS.git`. No commits yet.

## Dependencies

None. Phase 02 starts after the remote row is checked.
