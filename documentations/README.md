# Spectre PS documentation

Read these in order when changing behavior.

| File | What it decides |
| --- | --- |
| `ghostscript-baseline.md` | What Ghostscript 9.55.0 on this machine does, and what the 10.08 manual says. |
| `covered-and-not-covered.md` | Which Ghostscript jobs Spectre takes, and which it leaves. |
| `architecture.md` | Instance, front ends, graphics engine, device, library boundary. |
| `folder-structure.md` | Where files go, and when a directory is allowed to appear. |
| `public-api.md` | Exported types and functions. |
| `cli.md` | `spectreps` subcommands and exit codes. |
| `language.md` | PostScript subset, execution rules, errors, limits, banned operators. |
| `devices.md` | Pixmap, PPM, PNG, PDF rewrite, compare, validate. |
| `development.md` | Make targets and how a phase row gets checked. |

The checklist that tracks the work is `plans/v0.0.1/00-program.md`.
