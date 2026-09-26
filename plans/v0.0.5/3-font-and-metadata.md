# v0.0.5 - Font and metadata

> **Parent:** `plans/v0.0.5/00-program.md` - program ledger
> **Status:** planned
> **Estimated effort:** about two days

---

## Overview

Three packages have no benchmark at all. `internal/font` is 6,820 implementation lines and holds the standard 14 metrics, three encoding tables, the AGL name map, the Type 1 program decoder, and TrueType subsetting. `internal/pdfa` is 2,070 lines and holds the PDF/A and PDF/UA-2 metadata plus both preflights. `internal/engine` is 41 lines and holds `CompareBytes`.

`internal/font` has the most interesting untimed cost in the tree. `widths_data.go` declares 14 `map[string]Width` literals of roughly 220 entries each and `codes_data.go` declares 14 `map[byte]Width` literals of roughly 256 entries each. A map literal with more than 25 entries compiles to a generated init loop, so the whole set is built during package init, before any benchmark timer starts. `documentation/performance.md:176` records 2.6 ms of process startup for `spectreps version`, and this is the most likely reason. A benchmark inside the package cannot see package init cost, so this file measures the lookup paths instead and records the startup number from the v0.0.4 application table as the only evidence for init.

`internal/pdfa` is the second gap. `Preflight` (`internal/pdfa/preflight.go:73`) loops every object in the file, and `PreflightUA2` (`internal/pdfa/ua2_preflight.go:50`) runs a second, independent content-stream scanner over the same bytes at `internal/pdfa/ua2_content.go:27`. Neither is timed. Both run inside every PDF/A rewrite and every tagged write, so they are on the path of two of the five `RewritePDF` writers.

`internal/engine` is cheap to cover. `CompareBytes` (`internal/engine/compare.go:7`) is one byte loop and is already measured indirectly by `BenchmarkCompareFiles` in `spectreps/bench_test.go:277`, but the direct measurement costs one row and it is the only function in the package.

## Executive summary

Phase 3 gives each of the three packages a first benchmark. The font benchmarks cover the standard 14 lookup, the encoding tables, the AGL map, the Type 1 program load, and TrueType subsetting, all of which run inside a text job. The PDF/A benchmarks cover `Preflight` at each mode, `PreflightUA2`, and the XMP packet build. The engine benchmark covers the byte compare. None of these packages changes.

## Phase 1: Font

- [x] 1.1 `internal/font/bench_test.go` adds `BenchmarkStandard14Lookup`, which resolves all 14 standard 14 names through `Standard14` (`internal/font/font.go:48`) and then runs both advance lookups on each resolved metrics, so the font map hit and the two advance map hits are all inside the timed loop. Proof: `go test -count=1 -run '^$' -bench BenchmarkStandard14Lookup -benchmem ./internal/font` exited 0. Outcome on 2026-09-26: 379.6 ns/op and 0 allocs, so the standard 14 tables are not a memory cost. The advance lookups assert nothing, because Symbol and ZapfDingbats have no A glyph.
- [x] 1.2 `BenchmarkEncodingLookup` walks all 256 codes of `EncodingStandard` through `GlyphName` (`internal/font/encodings.go:50`) and resolves them back through `GlyphCode` (`internal/font/encodings.go:62`), which is the pair the text job runs per shown code. Proof: `go test -count=1 -run '^$' -bench BenchmarkEncodingLookup -benchmem ./internal/font` printed one line. Outcome on 2026-09-26: 52444 ns/op and 0 allocs for 768 pairs, so 68 ns per pair.
- [x] 1.3 `BenchmarkAGLUnicode` resolves glyph names through `AGLUnicode` (`internal/font/agl.go:7`) over the `aglNames` table in `internal/font/agl_data.go`. Proof: `go test -count=1 -run '^$' -bench BenchmarkAGLUnicode -benchmem ./internal/font` exited 0. Outcome on 2026-09-26: 49.76 ns/op and 0 allocs for eight names.
- [x] 1.4 `BenchmarkLoadType1` loads a synthetic Type 1 program through `LoadType1` (`internal/font/type1.go:126`) and then reads one glyph through `Glyph` (`internal/font/type1.go:114`) per iteration, so the program parse and the charstring interpret are separated into two sub-benchmarks. Proof: `go test -count=1 -run '^$' -bench BenchmarkLoadType1 -benchmem ./internal/font` printed two lines. Outcome on 2026-09-26: `parse` 20970 ns/op and 32000 B/op over 163 allocs, `glyph` 497.0 ns/op and 648 B/op over 15 allocs. The parse is 42 times the interpret.
- [x] 1.5 `BenchmarkSubset` subsets the synthetic TrueType program from `internal/truetypesynth` through `Subset` (`internal/font/subset.go:59`) and resolves its tag through `SubsetTag` (`internal/font/subset.go:42`), which is what `RewritePDF` with `SubsetFonts` pays. Proof: `go test -count=1 -run '^$' -bench BenchmarkSubset -benchmem ./internal/font` printed two lines. Outcome on 2026-09-26: `subset` 2354 ns/op and 3032 B/op over 27 allocs, `tag` 284.5 ns/op and 8 B/op over 1 alloc.

## Phase 2: PDF/A and PDF/UA-2

- [x] 2.1 `internal/pdfa/bench_test.go` adds `benchSample`, which reads a checked-in file from `sampledata/` and skips when it is absent, matching the skip rule in `spectreps/bench_test.go:39`. Proof: `go test -count=1 -run '^$' -bench . -benchmem ./internal/pdfa` exited 0.
- [x] 2.2 `BenchmarkPreflight` runs `Preflight` (`internal/pdfa/preflight.go:73`) at `Mode4` over the compliant sample, and `BenchmarkPreflightUA2` runs `PreflightUA2` (`internal/pdfa/ua2_preflight.go:50`) over the PDF/UA-2 sample, so the object loop and the second content scanner are both timed. Proof: `go test -count=1 -run '^$' -bench 'BenchmarkPreflight|BenchmarkPreflightUA2' -benchmem ./internal/pdfa` printed two lines. Outcome on 2026-09-26: `BenchmarkPreflight` 1653991 ns/op and 4938 B/op over 12 allocs, `BenchmarkPreflightUA2` 371776 ns/op and 168936 B/op over 4512 allocs. The object loop is the most expensive single call outside a rewrite.
- [x] 2.3 `BenchmarkXMPPacket` builds the PDF/A and PDF/UA-2 XMP packets through `XMP` (`internal/pdfa/xmp.go:34`) and `UA2XMP` (`internal/pdfa/ua2.go:119`), which run on every PDF/A and every tagged write. Proof: `go test -count=1 -run '^$' -bench BenchmarkXMPPacket -benchmem ./internal/pdfa` printed two lines. Outcome on 2026-09-26: `pdfa4` 0.1534 ns/op and 0 allocs, because the packet is a constant string, and `ua2` 2596 ns/op and 9142 B/op over 13 allocs, because that packet is built per call.

## Phase 3: Engine

- [x] 3.1 `internal/engine/bench_test.go` adds `BenchmarkCompareBytes`, which compares two 1 MiB equal buffers and then the same pair with the last byte changed, so the match and the mismatch path are both timed. Proof: `go test -count=1 -run '^$' -bench BenchmarkCompareBytes -benchmem ./internal/engine` printed two lines. Outcome on 2026-09-26: `equal` 236164 ns/op and `last-byte` 241644 ns/op, both 0 allocs, and both at about 4400 MB/s. The mismatch on the final byte does not return early, which is the point of the pair.

## Phase 4: Closure

- [x] 4.1 Closure: `make lint` and `make test` exit 0, and the outcome is recorded in this row. Proof: `make lint` and `make test NPROC=4`. Outcome on 2026-09-26: both exit 0. `go test -p 4 ./...` reported ok for all 13 packages, `internal/font` in 0.008s, `internal/pdfa` in 0.018s, and `internal/engine` with no tests to run, since this file gave that package its first test file.

## Not in this phase

- A benchmark of `internal/font` package init. A `Benchmark` function cannot time work the runtime finishes before `main`, and `documentation/performance.md:176` already records the 2.6 ms startup number from the application side. Measuring the map literal cost needs a separate program, which is not this file.
- A benchmark of `internal/validation`, `internal/truetypesynth`, or `internal/type1synth`. All three exist to make tests work and none is on a product path. `internal/truetypesynth` is used here as a benchmark input and is not itself measured.
- Any change to a file in the three packages.

## Dependencies

- Phase 1 needs `internal/truetypesynth` as a TrueType input. The package has no dependency of its own, so there is no import cycle and no new module requirement.
- Phase 2 needs `internal/pdf`, which `internal/pdfa` already imports, and a checked-in sample from `sampledata/pdfa/`.
- Phase 3 needs nothing.
