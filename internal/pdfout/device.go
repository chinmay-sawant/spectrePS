// Package pdfout emits PDF path operators for the rewrite device.
package pdfout

import (
	"bytes"
	"image"
	"strconv"
	"strings"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

type recorder struct {
	buf      bytes.Buffer
	sawImage bool
}

func newRecorder() *recorder {
	return &recorder{buf: bytes.Buffer{}, sawImage: false}
}

// DrawImage records that an image was seen. The rewrite recorder cannot write
// image pixels, so Emit refuses the page instead of dropping the image.
func (rec *recorder) DrawImage(_ image.Image, _ graphics.Matrix, _ float64) {
	rec.sawImage = true
}

func (rec *recorder) Stroke(pts []graphics.Point, width, red, green, blue float64) {
	if len(pts) == 0 {
		return
	}
	rec.writeColor(red, green, blue)
	rec.writeOp(formatNum(width), "w")
	rec.writePath(pts)
	rec.writeOp("S")
}

func (rec *recorder) Fill(pts []graphics.Point, red, green, blue float64, evenOdd bool) {
	if len(pts) == 0 {
		return
	}
	rec.writeColor(red, green, blue)
	rec.writePath(pts)
	opName := "f"
	if evenOdd {
		opName = "f*"
	}
	rec.writeOp(opName)
}

func (rec *recorder) writeColor(red, green, blue float64) {
	rec.writeOp(formatNum(red), formatNum(green), formatNum(blue), "RG")
	rec.writeOp(formatNum(red), formatNum(green), formatNum(blue), "rg")
}

func (rec *recorder) writePath(pts []graphics.Point) {
	for _, point := range pts {
		opName := "l"
		if point.Move {
			opName = "m"
		}
		rec.writeOp(formatNum(point.X), formatNum(point.Y), opName)
	}
}

func (rec *recorder) writeOp(parts ...string) {
	rec.buf.WriteString(strings.Join(parts, " "))
	rec.buf.WriteByte('\n')
}

// formatNum prints a decimal without an exponent.
func formatNum(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// bytes returns the operators. An empty recording is a non-nil empty slice.
func (rec *recorder) bytes() []byte {
	if rec.buf.Len() == 0 {
		return []byte{}
	}
	out := make([]byte, rec.buf.Len())
	copy(out, rec.buf.Bytes())
	return out
}
