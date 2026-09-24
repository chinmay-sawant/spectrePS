# Spectre PS

Spectre PS is a Go CLI now and a Go library on the same functions. Package `spectreps` at `github.com/chinmay-sawant/spectrePS` is the API. `cmd/spectreps` only parses arguments, calls that package, and maps errors to exit codes. Do not import `internal/` from `cmd/spectreps`. Do not start or link system Ghostscript.

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
