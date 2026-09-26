# Architecture

## Instance

Ghostscript's C entry point is one instance: create it, init it, run a program, exit, delete it. The `gs` binary is a small caller of that API. Spectre copies the lifecycle and not the C types.

`New` allocates an `Instance`. Job methods run on that value. `Close` releases it. `Close` is safe when no job ran. A second `Close` is safe. Callers may hold more than one instance. There is no process-wide singleton. Ghostscript's old one-instance limit is not copied.

`cmd/spectreps` imports `internal/cli` only. `internal/cli` imports package `spectreps` and calls that public API. An external test package, `spectreps_test`, imports the same public API. Tests that need a private seam wait until that seam is the bug under test, and then they live next to the private package.

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

## Where code lives

Package `spectreps` stays small: options, results, errors, and methods that delegate to the interpreter packages under `internal/`.

| Path | Owns |
| --- | --- |
| `spectreps/` | The exported API: 16 implementation files. |
| `internal/engine/` | The session handle and the file byte compare. |
| `internal/cli/` | Flags, exit codes, the `gs` argv scanner, and the output encoders. |
| `cmd/spectreps/main.go` | The process entry. Calls `internal/cli` and nothing else. |
| `internal/ps/` | The PostScript scanner, object model, stacks, and operators. |
| `internal/graphics/` | Matrix, path, color, blend, clip, and the RGB pixmap device. |
| `internal/pdf/` | Xref, objects, stream filters, images, color spaces, fonts, the content interpreter, and the structure tree reader. |
| `internal/pdfout/` | The PDF writers: the level 0 path writer, the level 1 to 5 pass-through writer, the packed writer, and the bitmap PDF writer. |
| `internal/psout/` | The PDF-to-PostScript writer. |
| `internal/pdfa/` | The PDF/A-4 and PDF/UA-2 metadata, ICC profile, and preflights. |
| `internal/tag/` | The structure tree recorder, reading order inference, and the tagged write. |
| `internal/font/` | Advances, encodings, glyph names, the Type 1 program decoder, and TrueType subsetting. |
| `internal/type1synth/` | A synthetic Type 1 program the tests embed. |
| `internal/truetypesynth/` | A synthetic TrueType program the tests embed. |
| `internal/validation/` | The validation corpus manifest reader and the corpus checker. |

There is no `internal/raster`. The RGB pixmap and every output encoder live in `internal/graphics` and `internal/cli`, and the level 0 PDF writer lives in `internal/pdfout`.

Directories are created with their first file. `internal/` is the right place for the interpreter because other modules must not import it. The public methods are what make that legal. A tree that is only `internal/` plus a main package would force a future caller to shell out.

## Interpreter state

The interpreter holds its whole mutable state on the `Interp` value: the operand, dictionary, and execution stacks, the graphics state, the loop depth, and the step counter. Nothing is stored in a package-level map keyed by interpreter, so two interpreters share nothing and nothing outlives the interpreter that owns it. A test covers this by running two interpreters and checking that the second one does not see the first one's definitions or transformation.

The `exit` stop condition is a private error type and not `*Error`. The interpreter re-tags every `*Error` with the operator the program invoked, so a shared `*Error` would be rewritten by every `exit` in every interpreter.

## Safety

Banned operators are installed and return `invalidaccess`, so a test can tell "disabled" from "unknown name". The list is in `documentation/language.md`.

Every job takes a `context.Context`. A cancelled context stops the job and returns `ctx.Err()`. The hard caps are in the same language file: stack depth, procedure nesting, path points, array and string size, executed objects, retained page bytes, pixel count, and page side. Crossing a cap returns `limitcheck`, except an oversized `array` or `string`, which returns `rangecheck` because the size is an operand.

The caps are a floor on what a program can consume, not a substitute for a deadline. A caller that cannot trust its input should also cancel the context.

Encrypted PDFs, unknown stream filters, and unsupported content operators return a `JobError`. The page is not replaced with a blank success.
