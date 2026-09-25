# Fixture README

Checked-in fixtures for the unit tests. Tests read them by relative path from
their package directory. Nothing here is copied from Ghostscript output.

# JPEG2000 fixtures

`opj_compress` is not installed on this machine, so Pillow 12.3.0 wrote both
files through its bundled OpenJPEG 2.5.4. The fixtures are 8 by 8, lossless,
and tiny. Run the command from this directory to reproduce them:

```sh
python3 - <<'EOF'
from PIL import Image

rgb = Image.new("RGB", (8, 8))
for y in range(8):
    for x in range(8):
        rgb.putpixel((x, y), (x * 32, y * 32, (x + y) * 16))
rgb.save("jpx-rgb.j2k", format="JPEG2000", irreversible=False, num_resolutions=3)

gray = Image.new("L", (8, 8))
for y in range(8):
    for x in range(8):
        gray.putpixel((x, y), (y * 32 + x * 4) % 256)
gray.save("jpx-gray.jp2", format="JPEG2000", irreversible=False, num_resolutions=3)
EOF
```

SHA-256:

- `jpx-rgb.j2k` `1614c6126ce69fa789155c37d25357f667d08c39235d22c986f50b324c916232`
- `jpx-gray.jp2` `798e95c7d993b3eeca283dfb5eb689c9a53f6670dbb7cdef85a937ffc3db9896`

`jpx-rgb.j2k` is a raw codestream. `jpx-gray.jp2` is a JP2 container. The RGB
pixel at `(x, y)` is `(x*32, y*32, (x+y)*16)`. The gray pixel at `(x, y)` is
`(y*32 + x*4) % 256`. The decoder tests rebuild the limit case from the RGB
fixture by rewriting the four SIZ size fields.

# Text fixture

`text-tj.ppm` is the page from `TestTjGlyphPixels`: 20 by 20 pixels at 72 dpi
with the synthetic `A` glyph in black on white. Running
`UPDATE_FIXTURES=1 go test ./internal/pdf -run TestTjGlyphPixels` rewrites it.
The font bytes come from the `synthFont` helper in `font_fixture_test.go`, not
from a checked-in font file.

SHA-256: `text-tj.ppm` `f42fa88995735407f44c5146d579bdf53d0b1332c63a7f0628a85cf70037357a`

# CCITT fixtures

`ccitt-g4.bin` and `ccitt-g3.bin` are raw strip bytes, not TIFF containers.
Pillow 12.3.0 wrote them with `gen_ccitt.py`:

```sh
python3 sampledata/fixtures/gen_ccitt.py sampledata/fixtures
```

SHA-256:

- `ccitt-g4.bin` `b9ea02cafa44593fb4b3b56e159e00c98a533b69f5176ae5ce03b8205f9f0274`
- `ccitt-g3.bin` `1e41f54b7b17df07b8316c7c06b3902908591cab0f6f2b7e008370cd3d1b9743`

# Other fixtures

- `gs-argv-input.pdf` is the two-page red and green input for `TestGSEndToEnd`.
- `line-bottom.ppm` is the expected PPM for the first raster case, the
  PostScript y-flip test in `spectreps/raster_test.go`.
- `text.pdf` is a hand-written two-line Helvetica PDF, the same page the
  extraction test builds. `spectreps text` prints `Hello` and `World`.
  SHA-256 `fc427fccbd290e1748841b6266f0d4a9d8e045075631a019e5b2c18a4ebab111`.
