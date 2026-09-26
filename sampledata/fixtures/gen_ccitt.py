#!/usr/bin/env python3
"""Regenerate the CCITT strip fixtures for internal/pdf.

Usage, from the repository root:

    python3 sampledata/fixtures/gen_ccitt.py sampledata/fixtures

Writes the raw TIFF strip bytes, which is what a PDF /CCITTFaxDecode stream
carries. A TIFF strip holds the fax codes with the opposite bit polarity to a
PDF stream, so the Pillow image below is the inverse of the pattern the PDF
decoder returns with /BlackIs1 false.
"""
import hashlib
import sys

from PIL import Image

# The pattern the PDF stream decodes to with /BlackIs1 false:
# 0 is black, 255 is white, row 0 first.
PDF_ROWS = [
    [0, 0, 0, 0, 0, 0, 0, 0],                  # all black
    [255, 255, 255, 255, 255, 255, 255, 255],  # all white
    [0, 255, 0, 255, 0, 255, 0, 255],          # alternating
]


def build_image():
    img = Image.new("1", (8, 3), 0)
    for y, row in enumerate(PDF_ROWS):
        for x, value in enumerate(row):
            img.putpixel((x, y), 255 - value)
    return img


def strip_bytes(path):
    data = open(path, "rb").read()
    with Image.open(path) as img:
        offset = img.tag_v2[273]
        count = img.tag_v2[279]
    if not isinstance(offset, tuple):
        offset = (offset,)
    if not isinstance(count, tuple):
        count = (count,)
    return b"".join(data[at:at + size] for at, size in zip(offset, count))


def main():
    out = sys.argv[1]
    img = build_image()
    for name, compression in (("ccitt-g4.bin", "group4"), ("ccitt-g3.bin", "group3")):
        tiff = "/tmp/%s.tif" % compression
        img.save(tiff, format="TIFF", compression=compression)
        raw = strip_bytes(tiff)
        open(out + "/" + name, "wb").write(raw)
        print(name, len(raw), raw.hex(), hashlib.sha256(raw).hexdigest())


if __name__ == "__main__":
    main()
