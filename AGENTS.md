# Spectre PS

Spectre PS is a Go CLI now and a Go library on the same functions. Package `spectreps` at `github.com/chinmay-sawant/spectrePS/spectreps` is the API. `cmd/spectreps` is the process entry and calls `internal/cli`. That package parses arguments, calls the public library, and maps errors to exit codes. `internal/cli` does not import `internal/engine` or a later interpreter package. Do not start or link system Ghostscript.

The product reads PostScript and PDF. It rasterizes pages, rewrites a new PDF, reports interpreter errors, and compares bytes. Printer drivers, PCL, and XPS are out of this ledger.

## Read before editing

- How Spectre is written, and the copyright record: `documentation/copyright-and-rewrite.md`
- Measured Ghostscript behavior: `documentation/ghostscript-baseline.md`
- Which Ghostscript jobs Spectre takes: `documentation/covered-and-not-covered.md`
- Tests for those jobs: `documentation/test.md`
- Runtime shape: `documentation/architecture.md`
- Tree: `documentation/folder-structure.md`
- Exported functions: `documentation/public-api.md`
- Commands and exit codes: `documentation/cli.md`
- PostScript subset, errors, limits, banned operators: `documentation/language.md`
- Raster, rewrite, compare, validate: `documentation/devices.md`
- Make targets and checklist rules: `documentation/development.md`
- Active ledger: `plans/v0.0.1/00-program.md`

## While implementing

1. Work the next unchecked row in the active phase file under `plans/v0.0.1/`.
2. A row is one change or one proof. Mark `[x]` only after that proof passes. On a closure row, write the command and the outcome.
3. When a phase changes Go code, run `make lint` and `make test` before marking the phase complete. A documentation-only change skips both.
4. Keep one open copy of a row. A deferred item is `[~]` in `plans/v0.0.1/10-deferred.md`, with the reason and the phase that has to land first.

## Code structure

Keep each `.go` file at or under 2,000 lines. Count test files. A file past that limit is overflow. Record it in `scripts/file-size-allowlist.txt` as the repo-relative path, a tab, and the exact line count. `make size-check` reads that list. It fails when an over-limit file is absent, when a listed file is missing or back under the limit, or when the recorded count disagrees with the file. `make lint` runs the check. The list is empty because every Go file on this tree is under the limit.

## Writing

Apply `skills/unslop/SKILL.md` before plans, docs, knowledge-base pages, PR text, issue text, commit messages, and replies. Sentence case headings. Straight quotes. Periods and commas.

## Git and reviews

- Remote: `https://github.com/chinmay-sawant/spectrePS.git`
- Integration branch: `master`
- Issue text: `skills/PR/ISSUE_TEMPLATE.md`
- Pull request text: `skills/PR/PR_TEMPLATE.md`
- Issue and PR comments: `skills/PR/COMMENT_TEMPLATE.md`

## Job rules

- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` return `invalidaccess`.
- `CompareRaster` compares RGB pixel bytes. `CompareFiles` compares file bytes. PNG and PDF container bytes are a different question, and they are not the raster equality check.
- `RewritePDF` writes a new PDF. Two calls with the same input return the same bytes. Input bytes are not the expected output.
- `validate` stops on the first error. It does not certify PDF/A.
