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
