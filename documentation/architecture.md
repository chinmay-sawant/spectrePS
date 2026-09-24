# Architecture

## Instance

Ghostscript's C entry point is one instance: create it, init it, run a program, exit, delete it. The `gs` binary is a small caller of that API. Spectre copies the lifecycle and not the C types.

`New` allocates an `Instance`. Job methods run on that value. `Close` releases it. `Close` is safe when no job ran. A second `Close` is safe. Callers may hold more than one instance. There is no process-wide singleton. Ghostscript's old one-instance limit is not copied.

`cmd/spectreps` imports package `spectreps` and nothing under `internal/`. An external test package, `spectreps_test`, imports the same public API. Tests that need a private seam wait until that seam is the bug under test, and then they live next to the private package.

No cgo. No `os/exec` of `gs` or of `spectreps`.

## Two front ends, one graphics engine

PostScript and PDF do not share a parser.

The PostScript front end scans tokens and executes them with three stacks: operand, dictionary, and execution. Procedure bodies are the exception to immediate execution. `documentation/language.md` states the rule.

A PDF file is a graph of objects, an xref table, and a page tree. Page content streams use a different operator spelling (`m`, `l`, `c`, `S`, `f`) and a different set of stacks. Those operators call the same graphics engine the PostScript path operators call.

```
PostScript tokens ----+
                       +--> graphics state and current path --> device
PDF content stream ---+
```

The device does not see user space. Operators transform the path by the current transformation matrix, then hand device-space points to the device. The pixmap device flips y because PostScript's origin is the lower left and image row 0 is the top. The flip happens in that device, not in the interpreter.

A PDF rewrite device receives the same device-space marks and emits PDF operators. It does not round-trip the original content stream bytes.

## Where code will live

The root package stays small: options, results, errors, and methods that delegate.

| Path | Appears in | Owns |
| --- | --- | --- |
| `instance.go`, `errors.go`, `compare.go`, `postscript.go`, `pdf.go`, `raster.go` | phase 02 | Exported API. |
| `cmd/spectreps/main.go` | phase 02 | Flags, exit codes. |
| `internal/ps/` | phase 03 | Scanner, stacks, operators. |
| `internal/graphics/` | phase 04 | Matrix, path, color, line style. |
| `internal/raster/` | phase 04 | RGB pixmap and PPM writer. PNG encode can sit beside it. |
| `internal/pdf/` | phase 06 | Xref, objects, content stream, Flate decode. |
| `internal/pdfout/` | phase 07 | New PDF, Flate encode. |

Directories are created with their first file. `internal/` is the right place for the interpreter because other modules must not import it. The public methods are what make that legal. A tree that is only `internal/` plus a main package would force a future caller to shell out.

## Safety

Banned operators are installed and return `invalidaccess`, so a test can tell "disabled" from "unknown name". The list is in `documentation/language.md`.

Every job takes a `context.Context`. A cancelled context stops the job and returns `ctx.Err()`. Stack depth, pixel count, and path points have hard caps in the same language file. Crossing a cap returns `limitcheck`.

Encrypted PDFs, unknown stream filters, and unsupported content operators return a `JobError`. The page is not replaced with a blank success.
