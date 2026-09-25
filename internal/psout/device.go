// Package psout emits PostScript path operators for the ps2write device.
package psout

import (
	"bytes"
	"image"
	"strconv"
	"strings"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

type recorder struct {
	buf bytes.Buffer
}

func newRecorder() *recorder {
	return &recorder{buf: bytes.Buffer{}}
}

func (rec *recorder) Stroke(pts []graphics.Point, width, red, green, blue float64) {
	if len(pts) == 0 {
		return
	}
	rec.writeColor(red, green, blue)
	rec.writeOp(formatNum(width), "setlinewidth")
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

// DrawImage records that an image was seen. Emission waits for the image
// operators in the PostScript interpreter, so the body records nothing yet.
func (rec *recorder) DrawImage(pic image.Image, ctm graphics.Matrix, scale float64) {
	_ = pic
	_ = ctm
	_ = scale
}

// writeColor picks setgray when the three channels match, and setrgbcolor
// otherwise. PostScript carries color across paint operators, so one operator
// per mark is enough.
func (rec *recorder) writeColor(red, green, blue float64) {
	if red == green && green == blue {
		rec.writeOp(formatNum(red), "setgray")
		return
	}
	rec.writeOp(formatNum(red), formatNum(green), formatNum(blue), "setrgbcolor")
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
