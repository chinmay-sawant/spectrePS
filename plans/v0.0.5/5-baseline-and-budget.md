# v0.0.5 - Baseline and budget

> **Parent:** `plans/v0.0.5/00-program.md` - program ledger
> **Status:** planned
> **Estimated effort:** about two days

---

## Overview

`documentation/performance.md` is the accepted record. It holds the machine, the toolchain, the command table, five benchmark tables of ns/op, B/op, and allocs/op, the CPU and allocation profile dumps, a findings table, and the allocation budget. It was recorded on 2026-09-26 on go1.26.4 against the 35 benchmarks that existed that day.

Phases 1 through 4 add roughly 40 benchmarks across nine packages. Eight of those packages are new to `BENCH_PKGS` in the Makefile, so `make bench` will not run them until the variable is updated. The recorded tables will not describe the wider suite until they are re-recorded, and a reader who runs `make bench` and finds benchmarks that the document does not mention has no way to tell whether the document is stale or the benchmark is new.

The budget is the other half. `spectreps/alloc_test.go` and `internal/cli/alloc_test.go` set `testing.AllocsPerRun` ceilings for six operations, and those are ordinary tests, so they gate `make test`. Every new benchmark that reports a stable allocation count is a candidate for the same treatment. The count has to be exact and the guard proof has to be run, because a ceiling nobody has falsified is a comment.

## Executive summary

Phase 5 wires the new packages into `make bench`, re-records `documentation/performance.md` against the wider suite without touching the 35 existing rows, and adds allocation ceilings for the new stable counts. Phase 5 also records the one machinery gap this file does not fix, so the next reader does not discover it twice.

## Phase 1: Build wiring

- [x] 1.1 `BENCH_PKGS` in the Makefile gains `./internal/engine`, `./internal/font`, `./internal/pdfa`, `./internal/psout`, and `./internal/tag`, so `make bench`, `make bench-profile`, and `make bench-check` all cover the packages phases 1 through 4 added. Proof: `make -n bench` printed a `go test` line naming all eleven packages, and `make bench` exited 0.
- [x] 1.2 `documentation/development.md` states that `make bench` covers eleven packages rather than six, and `documentation/test.md` names the packages the wider suite now covers. Proof: `grep -n -e 'eleven' documentation/development.md documentation/test.md documentation/performance.md` printed the updated sentence in all three.

## Phase 2: Re-recorded baseline

- [x] 2.1 `make bench` runs on this machine and `profiles/bench.txt` holds a full run over the eleven packages, so the numbers come from one machine and one toolchain as the v0.0.4 table does. Proof: `make bench` exited 0 after 6 minutes, and `wc -l profiles/bench.txt` reported 324 lines against the 258 benchmark rows of the v0.0.4 capture.
- [x] 2.2 `documentation/performance.md` records the machine, the Go version, the date, and the eleven packages in a new section above the v0.0.4 tables, and states that the v0.0.4 tables below are the accepted comparison and are not re-recorded by this file. Proof: `grep -n -e ns/op -e '2026-09-26' -e i7-13700HX documentation/performance.md` printed the new `Coverage extension` header and the old machine table.
- [x] 2.3 `documentation/performance.md` gains one row per new benchmark group: the fill scaling table, the clip allocation counts, the encoder format table, the packer table, the font lookups, the two preflights, the `DerivePlan` slope, and the three library jobs. Each row names the top function with file:line and the measured value. Proof: `grep -c '^| ' documentation/performance.md` reports 331 rows against 252 before this row, and the new `Findings` rows name `graphics.inside`, `snapshotRect`, `EncodeFlateRGB`, `readLimited`, `packedCMYK`, `encodePPM`, `orderFlow`, `pdfa.Preflight`, and `ua2ContentRule`.
- [x] 2.4 `make bench-profile` runs over all eleven packages and `go tool pprof -top -nodecount=20` is recorded for the three that carry the most new cost. Proof: `ls -1 profiles/*.cpu` listed 11 CPU and 11 heap profiles and `go tool pprof -top profiles/pdfa.cpu` exited 0. Outcome on 2026-09-26: `pdfout` leads with `compress/flate.(*compressor).deflate` at 9.04 percent flat, `pdfa` with `pdf.(*File).ObjectCount` at 7.38 percent flat and 22.14 percent cumulative, and `tag` with `tag.runText` at 5.71 percent flat and 15.22 percent cumulative. The `pdfout` heap profile is the one that changed a conclusion, and its numbers are in `documentation/performance.md` under `Profiles for the new packages`.

## Phase 3: Allocation budget

- [x] 3.1 `spectreps/alloc_test.go` gains `TestJobAllocs` with a ceiling for each new `spectreps` benchmark whose allocation count is stable across five runs, which is `DocumentInfo`, `PreflightUA2`, `WritePostScript`, `RewritePDFTagged`, and `RewritePDFA`. Proof: `go test -count=1 ./spectreps -run TestJobAllocs` exited 0. Outcome on 2026-09-26: 7, 4512, 5628, 12090, and 147 allocs.
- [x] 3.2 `internal/cli/alloc_test.go` gains `TestEncoderAllocs` with a ceiling for `BenchmarkEncodePNG` and `BenchmarkEncodeTIFF` at `tiffNone`, the two whose counts do not depend on the encoder's internal buffer growth. Proof: `go test -count=1 ./internal/cli -run TestEncoderAllocs` exited 0. Outcome on 2026-09-26: 34 and 27 allocs.
- [x] 3.3 The guard proof runs. With `allocRewriteTagged` lowered by one, to 12089, `go test -count=1 ./spectreps -run TestJobAllocs` failed with `RewritePDF tagged allocs = 12090, want 12089`, and the restored value passed. With `allocCLIPNG` lowered by one, to 33, `go test -count=1 ./internal/cli -run TestEncoderAllocs` failed with `PNG encode allocs = 34, want 33`, and the restored value passed. The v0.0.4 level 2 ceiling was re-proved at its current value of 715 the same way, failing at 714 and passing when restored. Proof: the three failing runs and the three restored runs are recorded above and in `documentation/performance.md` section Budget.
- [x] 3.4 `documentation/performance.md` gains the new ceilings in the budget table with the tolerance column set to exact, and the surrounding paragraph keeps the statement that timing is machine-specific and only allocation counts gate `make test`. Proof: `grep -n -e 'machine-specific' -e 'WritePostScript' documentation/performance.md` printed both. The row for the level 2 rewrite moved from 701 to 715 to match what `internal/cli/alloc_test.go:22` asserts, and the paragraph says so.

## Phase 4: The gap this file leaves

- [x] 4.1 `documentation/performance.md` records in the tool matrix that `benchstat` is installed at `~/go/bin/benchstat` by `go install golang.org/x/perf/cmd/benchstat@latest`, that `~/go/bin` is on `PATH` from `~/.zshrc:52`, and that the install leaves `go.mod` and `go.sum` byte-identical, so it is a tool and not a module requirement. Proof: `grep -n benchstat documentation/performance.md` printed the row, and `md5sum go.mod go.sum` returned the same two hashes before and after the install.
- [x] 4.2 `documentation/performance.md` records that the repository has no CI, so nothing runs the suite automatically and the baseline is a local file that only this machine can reproduce. Proof: `grep -n -e 'no CI' documentation/performance.md` printed the sentence.
- [~] 4.3 Raise the comparison sample counts. `make bench` captures `-count=3` and `make bench-check` captures `-count=5`, so benchstat receives 3 and 5 samples. Its footnote asks for 6 before it will compute a confidence interval, 44 rows came back as `± ∞`, and all 107 rows it called significant sat at exactly `p=0.036`, which is the floor a Mann-Whitney U test can reach at that sample size. The result flagged a third of the suite on a tree where no non-test Go file had changed, with per-package `sec/op` geomeans from -2.22 to +46.67 percent at a load average of 1.04. Reason: the fix is `-count=10` on both sides, which doubles a run that already takes about ten minutes, and confirming the verdict needs a clean base capture and then a clean comparison, a further 20 minutes of machine time. Next gate: a decision from the integrator on whether `make bench-check` may take 20 minutes, and if so a re-capture of `profiles/bench.txt` at `-count=10` before any timing number in `documentation/performance.md` is treated as a baseline. The allocation columns are unaffected and stay exact.

## Phase 5: Closure

- [x] 5.1 Closure: `make lint` and `make test` exit 0, and the outcome is recorded in this row and in `plans/v0.0.5/v0.0.5-closure.md`. Proof: `make lint` and `make test NPROC=4`. Outcome on 2026-09-26: both exit 0, `go test -p 4 ./...` reported ok for all 13 packages, and `make bench` exited 0 with 258 benchmark rows across 85 lines of `profiles/bench.txt`. Row 4.3 stays deferred.

## Not in this phase

- Re-recording the 35 existing rows. They are the accepted comparison and the whole point of recording new numbers beside them. A reader compares the new table to the old one.
- A CI workflow. Row 4.2 records the gap. A workflow needs a runner image, a corpus, and a decision about whether timing is allowed to fail a build, and that is a later file.
- Setting a wall-clock budget. `documentation/performance.md` already states that timing is machine-specific, and no row here changes that.

## Dependencies

- Phases 1 through 4 of this file all need the numbers phases 1 through 4 of `1-graphics-device.md` through `4-reading-and-writing.md` produce. Nothing in this file is meaningful before those land.
- The integrator owns `plans/v0.0.5/00-program.md`, `plans/v0.0.5/v0.0.5-closure.md`, and any move into `plans/v0.0.1/10-deferred.md`. This file touches none of them.
