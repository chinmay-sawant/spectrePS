# Spectre PS documentation

Read these in order when changing behavior.

| File | What it decides |
| --- | --- |
| `ghostscript-baseline.md` | What Ghostscript 9.55.0 on this machine does, and what the 10.08 manual says. |
| `copyright-and-rewrite.md` | Why Spectre is an independent implementation, and the copyright, trademark, and patent record. |
| `covered-and-not-covered.md` | Which Ghostscript jobs Spectre takes, and which it leaves. |
| `features.md` | The feature inventory: what works now, and what waits. |
| `pdf-compatibility.md` | PDF version reporting, corpus refusals, and the boundary of the compliance checks. |
| `test.md` | The tests those covered jobs need. |
| `architecture.md` | Instance, front ends, graphics engine, device, library boundary. |
| `folder-structure.md` | Where files go, and when a directory is allowed to appear. |
| `public-api.md` | Exported types and functions. |
| `cli.md` | `spectreps` subcommands and exit codes. |
| `language.md` | PostScript subset, execution rules, errors, limits, banned operators. |
| `devices.md` | Pixmap, PPM, PNG, PDF rewrite, compare, validate. |
| `fonts.md` | Where advances, encodings, glyph names, and the Type 1 decoder come from. |
| `gs-argv-grammar.md` | Every `spectreps gs` switch the allowlist accepts, and every message it prints. |
| `gs-argv-mapping.md` | The same mapping as a readable `gs` device to `spectreps` command table. |
| `performance.md` | Measured baselines, profiles, and the allocation budgets that gate `make test`. |
| `benchmark.md` | The recorded `make bench` run, every benchmark with ns/op, B/op, and allocs/op. |
| `reference-proofs.md` | The Ghostscript and veraPDF cross-checks, run by hand and never in `make test`. |
| `development.md` | Make targets and how a phase row gets checked. |

`plans/v0.0.5/00-program.md` tracks benchmark work and the PDF compatibility
corpus. The released tag remains v0.0.4. Deferred product work stays in
`plans/v0.0.1/10-deferred.md`.

The released tag is v0.0.4, which is the version the command reports. Documentation that says a feature "landed in v0.0.4" means it shipped with that tag.
