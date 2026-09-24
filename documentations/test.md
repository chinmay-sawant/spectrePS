# Tests for covered jobs

These are the tests for the jobs in `documentations/covered-and-not-covered.md`. Each bullet is one case. The expected result is the contract in `documentations/language.md`, `documentations/devices.md`, `documentations/cli.md`, and `documentations/public-api.md`.

Tests call package `spectreps` or the `spectreps` binary. They do not run `/usr/bin/gs`. Fixtures are Spectre output, checked in under `testdata/` when the first raster or PDF case lands. PNG file bytes are not an equality oracle. The oracle is `PageImage`, or a PPM raw body when the header is part of the case.

External tests use `package spectreps_test`, so they only see the exported API.

## Library and CLI

- `New` returns a non-nil instance. `Close` returns nil. A second `Close` on the same instance returns nil.
- Two instances from two `New` calls both run. There is no process-wide singleton.
- `Version` is `0.0.1` until a tag bumps that constant. `spectreps version` prints it and exits 0.
- `cmd/spectreps` imports only `github.com/chinmay-sawant/spectrePS`. No Go file imports `os/exec` or uses cgo.
- Unknown command, unknown flag, and a missing input file exit 2.
- A missing input path that the program tries to read exits 3.
- `raster` and `rewrite` without `-o` exit 2, including while the job itself is still `ErrNotImplemented`.
- A cancelled context passed to `RunPostScript`, `OpenPDF`, `RasterizePage`, or `RewritePDF` returns `ctx.Err()` and no partial success.

## PostScript subset

- Scanner accepts an int, a real, an executable name, a literal name, a `%` comment, a parenthesis string with `\n`, `\r`, `\t`, `\\`, `\(`, `\)`, and `\ddd`, and a hex string. An odd final hex nibble is padded with 0. An integer token outside int32 is `rangecheck`.
- `{ { 1 2 add } }` is one executable array whose only element is an executable array. Unmatched braces are `syntaxerror`.
- Top-level `1 2 add` leaves 3. `{ 1 2 add }` leaves a procedure. `{ 1 2 add } exec` leaves 3. `{ { 1 2 add } } exec` leaves the inner procedure and does not run `add`.
- A procedure built while a name has one definition, then the name is redefined, runs the new definition. Lookup happens at execution time.
- `div` pushes a real. Division by zero is `undefinedresult`. Integer overflow on `add` is `rangecheck`. `copy` with a non-integer top operand is `typecheck`.
- `eq` treats int 1 and real 1.0 as equal. `eq` on two distinct arrays with the same elements is false. `eq` on the same array object is true.
- Dictionary `forall` walks entries in insertion order.
- `def` into `systemdict` is `invalidaccess`. `def` into `userdict` succeeds. `end` when only `systemdict` remains is `dictstackunderflow`. `]` with no mark is `unmatchedmark`. `exit` outside a loop is `invalidexit`. `show` is `undefined`.
- Operand stack past 8192 is `stackoverflow`. Execution stack past 500, dictionary stack past 20, and procedure nesting past 128 are `limitcheck`.
- `for` with a zero increment is `rangecheck`.

## Raster

- Default options produce a 612 by 792 RGB image at 72 dpi. Stride is `Width * 3`.
- On a 200 by 200 point page at 72 dpi, `0 0 moveto 100 0 lineto stroke` paints the bottom row. `currentpoint` after `0 0 moveto` is 0, 0 in user space. Row 0 of the pixmap stays the top of the page.
- `stroke`, `fill`, and `eofill` write RGB pixels. `eofill` uses the even-odd rule.
- `showpage` appends one image and clears the path. Two `showpage` calls return two images.
- A program that paints nothing and never calls `showpage` returns one blank page. A program that paints and never calls `showpage` returns one image.
- `translate`, `scale`, `rotate`, and `concat` change where the same path lands. `gsave` then `grestore` restores the matrix and the path. `gsave` past depth 32 is `limitcheck`.
- A page above 40000000 pixels, or a side above 20000 pixels, returns `limitcheck` and does not allocate that pixmap. A letter page at 600 dpi is over the cap. A letter page at 300 dpi is under it.
- `spectreps raster -o out.ppm in.ps` writes a P6 file. The header is `P6\n{width} {height}\n255\n`. The body matches `PageImage.Pixels`.
- `spectreps raster -o out.png in.ps` writes a PNG that decodes to the same pixels as the PPM from the same program. The test compares decoded pixels, not the PNG bytes.
- Two pages and an `-o` path with no `%d` exit 2. `%d` is the one-based page number.

## Pixel compare

- `CompareRaster` on two equal images sets `Equal` true, `Offset` -1, and an empty `Reason`.
- Different `Width` sets `Reason` `width` and `Offset` -1. Different `Height` with equal width sets `Reason` `height`.
- The first differing RGB byte sets `Reason` `pixel` and `Offset` to that byte index in row-major order. Bytes in the stride padding are ignored.
- `spectreps compare raster` uses one `RunOptions` value for both files. Equal pixels exit 0. A mismatch exits 1 and prints `mismatch pixel N` or `mismatch width` on stdout. stderr is empty.
- The compare command does not start `gs`.

## PDF open and rasterize

- A fixture with a `%PDF-` header, a classic xref, and a Flate content stream opens. The page count is the page tree length.
- A fixture that uses an xref stream and a Flate object stream opens, and the page count is right.
- Content operators `m l c h re S s f f* n q Q w RG rg g G` paint through the same device as the PostScript path operators. A one-page path PDF and the PostScript program of the same marks compare equal with `CompareRaster`.
- `Tj`, `TJ`, `'`, `"`, and `Do` each return `JobError` with the operator name filled in. The page is not a blank success.
- An encrypted file returns `invalidaccess`. An unknown stream filter returns `undefined`. A truncated xref returns `JobError`.
- `RasterizePage` with a negative index, or an index past the last page, returns `rangecheck`.
- `spectreps raster -o out.ppm in.pdf` writes the P6 file for a path-only fixture.

## PDF rewrite

- `RewritePDF` on a document this module can rasterize returns a PDF. Opening that PDF and rasterizing page 0 matches `RasterizePage` of the input, via `CompareRaster`.
- `DefaultRewriteOptions` Flate-compresses page content streams. `CompressStreams` false leaves those streams uncompressed. Both outputs still match the input pixels.
- Two `RewritePDF` calls on the same input return buffers `CompareFiles` reports equal. The output contains no wall-clock timestamp.
- The rewritten bytes are not required to equal the input bytes, and they are not compared with Ghostscript `pdfwrite`.
- `spectreps rewrite` without `-o` exits 2. `-compress=false` selects the uncompressed option. A successful rewrite exits 0.

## Validate

- `spectreps validate` on a subset PostScript program exits 0.
- `spectreps validate` on a PostScript `stackunderflow` exits 1. stderr is one line, `Error: /stackunderflow in add at <file>:<line>:<col>`.
- `spectreps validate` on a phase-06 PDF fixture exits 0. A truncated xref exits 1 with `JobError`. An encrypted file exits 1 with `invalidaccess`.
- `validate` does not write an output file and does not write PDF/A metadata.

## File access ban

- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` are defined and return `invalidaccess`.
- `spectreps validate` on a program that calls `deletefile` exits 1 with `invalidaccess`. A temp file created next to the input is still there after the command.
- A string path starting with `%pipe%` never runs, because `file` returns `invalidaccess` before it reads the path.
- No test, and no operator under test, opens a network connection.

## File byte compare

These cases belong to the library and the `compare bytes` command. Ghostscript has no compare command. Spectre does.

- Two equal slices, including two empty slices, set `Equal` true, `Offset` -1, and an empty `Reason`.
- The first differing byte sets `Reason` `byte` and `Offset` to that index.
- When one slice is a prefix of the other, `Offset` is the shorter length and `Reason` is `length`.
- `spectreps compare bytes` exits 0 for equal files and 1 for a mismatch, printing `mismatch byte N` on stdout.
