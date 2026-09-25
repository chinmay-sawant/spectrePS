package graphics

import (
	"image"
	"math"
)

// Clip is one clip path in device pixels. EvenOdd selects the even-odd rule.
// A mark survives the clip when its pixel center falls inside every path.
type Clip struct {
	Pts     []Point
	EvenOdd bool
}

// ClipMarker is implemented by markers that clip later marks. The Marker
// interface keeps its three methods, so a marker without this seam refuses
// W and W* with undefined. The pixmap implements it and the rewrite recorder
// does not.
type ClipMarker interface {
	// StrokeClipped strokes a path through the clip region.
	StrokeClipped(clips []Clip, pts []Point, width, red, green, blue float64)
	// FillClipped fills an interior through the clip region.
	FillClipped(clips []Clip, pts []Point, red, green, blue float64, evenOdd bool)
	// DrawGlyphClipped blends a glyph mask through the clip region.
	DrawGlyphClipped(clips []Clip, mask *image.Alpha, originX, originY int, red, green, blue float64)
	// DrawImageClipped stamps an image through the clip region.
	DrawImageClipped(clips []Clip, pic image.Image, ctm Matrix, scale float64)
}

// StrokeClipped strokes a path and puts back every mark pixel outside the clip.
// The device snapshot covers the path bounds, so the clipped-out pixels return
// to their prior bytes, which is exact under any later blend model.
func (p *Pixmap) StrokeClipped(clips []Clip, pts []Point, width, red, green, blue float64) {
	if width < 0 {
		width = -width
	}
	rect := p.pathRect(pts, width/halfWidth)
	before := p.snapshotRect(rect)
	p.Stroke(pts, width, red, green, blue)
	p.restoreOutside(clips, rect, before)
}

// FillClipped fills an interior and puts back every mark pixel outside the clip.
func (p *Pixmap) FillClipped(clips []Clip, pts []Point, red, green, blue float64, evenOdd bool) {
	rect := p.pathRect(pts, 0)
	before := p.snapshotRect(rect)
	p.Fill(pts, red, green, blue, evenOdd)
	p.restoreOutside(clips, rect, before)
}

// DrawGlyphClipped blends a glyph mask and puts back every mark pixel outside
// the clip. Mask row 0 is the top row, as in DrawGlyph.
func (p *Pixmap) DrawGlyphClipped(clips []Clip, mask *image.Alpha, originX, originY int, red, green, blue float64) {
	if mask == nil {
		return
	}
	rect := p.glyphRect(mask, originX, originY)
	before := p.snapshotRect(rect)
	p.DrawGlyph(mask, originX, originY, red, green, blue)
	p.restoreOutside(clips, rect, before)
}

// DrawImageClipped stamps an image and puts back every mark pixel outside the
// clip. The image unit square maps through ctm, then scales, as in DrawImage.
func (p *Pixmap) DrawImageClipped(clips []Clip, pic image.Image, ctm Matrix, scale float64) {
	rect, ok := p.imageRect(pic, ctm, scale)
	if !ok {
		return
	}
	before := p.snapshotRect(rect)
	p.DrawImage(pic, ctm, scale)
	p.restoreOutside(clips, rect, before)
}

// pathRect returns the pixel rectangle that holds every point plus pad.
// The rectangle is clamped to the page.
func (p *Pixmap) pathRect(pts []Point, pad float64) image.Rectangle {
	if len(pts) == 0 {
		return image.Rect(0, 0, 0, 0)
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, pt := range pts {
		minX = math.Min(minX, pt.X)
		maxX = math.Max(maxX, pt.X)
		minY = math.Min(minY, pt.Y)
		maxY = math.Max(maxY, pt.Y)
	}
	col0 := clampInt(int(math.Floor(minX-pad)), p.w)
	col1 := clampInt(int(math.Ceil(maxX+pad)), p.w)
	row0 := p.h - clampInt(int(math.Ceil(maxY+pad)), p.h)
	row1 := p.h - clampInt(int(math.Floor(minY-pad)), p.h)
	if row0 > row1 {
		row0, row1 = row1, row0
	}
	return image.Rect(col0, row0, col1, row1)
}

// glyphRect returns the pixel rectangle a glyph mask touches.
func (p *Pixmap) glyphRect(mask *image.Alpha, originX, originY int) image.Rectangle {
	bounds := mask.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	row0 := p.h - height - originY
	return image.Rect(originX, row0, originX+width, row0+height)
}

// imageRect returns the pixel rectangle a mapped image touches. The bool is
// false when the image does not map to any pixel, matching DrawImage.
func (p *Pixmap) imageRect(pic image.Image, ctm Matrix, scale float64) (image.Rectangle, bool) {
	if pic == nil {
		return image.Rect(0, 0, 0, 0), false
	}
	bounds := pic.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return image.Rect(0, 0, 0, 0), false
	}
	imageToDevice := Concat(ctm, Matrix{A: scale, B: 0, C: 0, D: scale, E: 0, F: 0})
	if _, ok := imageToDevice.Invert(); !ok {
		return image.Rect(0, 0, 0, 0), false
	}
	col0, col1, row0, row1 := p.imageSpan(imageToDevice)
	if col0 > col1 {
		col0, col1 = col1, col0
	}
	if row0 > row1 {
		row0, row1 = row1, row0
	}
	return image.Rect(col0, row0, col1, row1), true
}

// snapshotRect copies one pixel rectangle for a later restore. The rectangle
// is clamped to the page and row major.
func (p *Pixmap) snapshotRect(rect image.Rectangle) []byte {
	rect = rect.Intersect(image.Rect(0, 0, p.w, p.h))
	if rect.Empty() {
		return nil
	}
	rowLen := rect.Dx() * bytesPerPixel
	out := make([]byte, 0, rowLen*rect.Dy())
	for row := rect.Min.Y; row < rect.Max.Y; row++ {
		start := row*p.w*bytesPerPixel + rect.Min.X*bytesPerPixel
		out = append(out, p.pix[start:start+rowLen]...)
	}
	return out
}

// restoreOutside puts back every snapshot pixel whose center misses the clip.
func (p *Pixmap) restoreOutside(clips []Clip, rect image.Rectangle, before []byte) {
	rect = rect.Intersect(image.Rect(0, 0, p.w, p.h))
	if before == nil || rect.Empty() {
		return
	}
	subs, rules := clipSubpaths(clips)
	rowLen := rect.Dx() * bytesPerPixel
	for row := rect.Min.Y; row < rect.Max.Y; row++ {
		for col := rect.Min.X; col < rect.Max.X; col++ {
			posX := float64(col) + pixelCenter
			posY := float64(p.h-1-row) + pixelCenter
			if clipInside(subs, rules, posX, posY) {
				continue
			}
			dst := row*p.w*bytesPerPixel + col*bytesPerPixel
			src := (row-rect.Min.Y)*rowLen + (col-rect.Min.X)*bytesPerPixel
			copy(p.pix[dst:dst+bytesPerPixel], before[src:src+bytesPerPixel])
		}
	}
}

// clipSubpaths flattens each clip path once for the pixel walk.
func clipSubpaths(clips []Clip) ([][][]Point, []bool) {
	subs := make([][][]Point, len(clips))
	rules := make([]bool, len(clips))
	for idx, clip := range clips {
		subs[idx] = subpaths(clip.Pts)
		rules[idx] = clip.EvenOdd
	}
	return subs, rules
}

// clipInside reports whether a device point survives every clip path.
func clipInside(subs [][][]Point, rules []bool, posX, posY float64) bool {
	for idx := range subs {
		if !inside(subs[idx], posX, posY, rules[idx]) {
			return false
		}
	}
	return true
}
