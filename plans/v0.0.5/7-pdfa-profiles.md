# PDF/A profile corpus

> **Status:** phase complete on the development tree. No release was cut.
> **Parent:** `plans/v0.0.5/6-pdf-compatibility.md` covers PDF version reporting.

PDF versions and PDF/A profiles are different labels. This phase adds one
verified compliant example for each PDF/A profile that veraPDF supports.
Spectre's ability to open a sample is recorded separately from its PDF/A
compliance result.

## Corpus

- [x] Pin representative PDF/A-1a, 1b, 2a, 2b, 2u, 3a, 3b, 3u, 4, 4e, and 4f files. `TestValidationManifest`, `TestValidationLicenses`, and `TestValidationCorpusPDFA` passed over the new manifest rows.
- [x] Add `scripts/pdfa-profiles.tsv` and `scripts/check-pdfa-profiles.py`. They read veraPDF's JSON verdict for each profile and three known noncompliant controls.
- [x] `documentation/pdf-compatibility.md` distinguishes PDF header versions, PDF/A profiles, and Spectre's page-structure results.

## Closure

- [x] `make lint` and `make test NPROC=4` passed on 2026-09-26. Lint found 0 over-limit Go files; all test packages passed.
- [x] `make validation-run` passed on 2026-09-26 with 83 pass, 0 fail, 0 skipped. `make pdfa-corpus-check` passed with 14 pass, 0 fail, 14 samples using veraPDF 1.30.2. Eleven samples were compliant under their named profiles and three controls were noncompliant as expected.
