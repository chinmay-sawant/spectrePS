# v0.0.3 - gs argv grammar

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** not started.
> **Estimated effort:** about 3 days for the written proposals, 10 to 15 days for the parser

---

## Overview

`documentation/gs-argv-mapping.md` maps the switches the current commands can express. The deferred row wants the full grammar. Go's `flag` package stops at the first positional argument and cannot mix `-sNAME=value` with file names, so a compatibility mode needs a hand-written scanner, not a second `FlagSet`.

## Executive summary

The mode is an allowlist, not a Ghostscript clone. A switch is accepted when it maps onto behavior Spectre already guarantees. It is accept-and-ignore when the behavior is always on. Anything that would silently change pixels is rejected with a named message. The work is a written proposal per family, then a scanner that routes to the existing commands.

## Phase 1: Written proposals

### 1.1 Entry shape and policy

- [x] A new `documentation/gs-argv-grammar.md` states the entry shape (`spectreps gs ...`), the allowlist policy, the accept-and-ignore list, and the exit codes. Proof (2026-09-25): `test -f documentation/gs-argv-grammar.md` exited 0, and the file carries the entry shape, the allowlist policy, the accept-and-ignore table, and the exit-code table in its own sections.

### 1.2 Device and output

- [x] The device section maps `-sDEVICE` names onto subcommands, including `tiff24nc`, and defines `raster -format ppm|png|jpeg|tiff` so the device wins over the `-o` suffix. The output section maps `-sOutputFile` and defines the rejected forms (`-`, `%stdout`, `%pipe%`, printf widths). Proof (2026-09-25): `grep -c '^## Devices$\|^## Output$' documentation/gs-argv-grammar.md` printed 2; the device table names `tiff24nc` and the output section lists the four rejected forms.

### 1.3 Page range and geometry

- [x] The page section maps `-dFirstPage` and `-dLastPage` onto `-pages`, including `-dFirstPage=N` to `-pages N-`, and names the rejected `-sPageList` forms. The geometry section maps `-r`, `-dDEVICEWIDTHPOINTS`, `-dDEVICEHEIGHTPOINTS`, and `-g` at 72 dpi. Proof (2026-09-25): `grep -c '^## Page range$\|^## Geometry$' documentation/gs-argv-grammar.md` printed 2; the page table carries `-dFirstPage=N` to `-pages N-` and the rejected `-sPageList` forms, and the geometry section states the 72 dpi rule for `-g`.

### 1.4 Batch, safety, input, and values

- [x] Sections cover the accept-and-ignore switches (`-dBATCH`, `-dNOPAUSE`, `-q`, `-dSAFER`, `-dFIXEDMEDIA`), the rejected safety switches, one input file or `-f`, the rejected `-c`, and the `-s`/`-d` value allowlist. Proof (2026-09-25): `grep -c '^## Accept and ignore$\|^## Input$\|^## The -s and -d value allowlist$' documentation/gs-argv-grammar.md` printed 3; the sections name the five ignored switches, `-dNOSAFER` and `-dDELAYSAFER`, `-f`, `-c`, and the value allowlist table.

## Phase 2: Parser

### 2.1 Dispatch and scanner skeleton

- [ ] `spectreps gs` dispatches into a new scanner in `internal/cli/gs.go`. With no allowlisted switch it rejects everything and exits 2. Proof: `go test -count=1 ./internal/cli -run TestGSRejectsAll`.

### 2.2 Families

- [ ] The scanner accepts `-sDEVICE` with `raster -format`, `-sOutputFile`, `-dFirstPage` and `-dLastPage`, `-r`, the point-size switches, the accept-and-ignore switches, and one input or `-f`, and routes them to the existing commands. The `-s`/`-d` value parser allows only known names. Proof: `go test -count=1 ./internal/cli -run TestGSDevice`, `TestRasterFormatFlag`, `TestGSOutputFile`, `TestGSPageRange`, `TestGSResolution`, `TestGSPageSize`, `TestGSIgnoredSwitches`, `TestGSInputFile`, and `TestGSParamSyntax`.

### 2.3 Page list subset

- [ ] `-sPageList` accepts a simple comma list of pages and ranges and maps it onto the `-pages` grammar. Even and odd selections, reverse order, and `LastPage < FirstPage` stay rejected. Proof: `go test -count=1 ./internal/cli -run TestGSPageList`.

## Phase 3: Closure

### 3.1 End-to-end and closure

- [ ] A `ps2pdf`-shaped run with `-sDEVICE=pdfwrite` against a checked-in fixture writes a PDF that opens and rasterizes. Proof: `go test -count=1 ./internal/cli -run TestGSEndToEnd`.
- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

The existing commands and flags, and the page-range grammar from v0.0.2. No new module.

## Not in this plan

- Inline PostScript (`-c`), systemdict, search paths, fonts, ICC, antialiasing knobs, printer devices, multiple inputs, stream outputs, and `@file`.
- `--help` parity and the `-Z` debug switches.
