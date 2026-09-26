package graphics

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"testing"
)

const (
	benchPageWidth  = 612
	benchPageHeight = 792
	benchPolyPoints = 200

	// The fill benchmarks use a 200 by 200 page rather than the letter page
	// above. Fill walks every pixel and calls inside for each one, so it costs
	// W*H*N, and a letter page makes the deeper paths too slow to run at the
	// -count=3 that make bench uses.
	benchFillSide = 200

	// benchGlyphSide is the coverage mask side the glyph benchmarks blend.
	benchGlyphSide = 64
)

// benchPoints builds a zigzag polyline across the page.
func benchPoints() []Point {
	pts := make([]Point, 0, benchPolyPoints)
	for i := range benchPolyPoints {
		pt := Point{X: float64(i) * 3, Y: float64((i * 7) % benchPageHeight), Move: i == 0}
		pts = append(pts, pt)
	}
	return pts
}

// benchSquare builds a 200 by 200 square path with a move to start it.
func benchSquare() []Point {
	return []Point{
		{X: 100, Y: 100, Move: true},
		{X: 300, Y: 100},
		{X: 300, Y: 300},
		{X: 100, Y: 300},
	}
}

// benchRGBA builds one solid RGBA image.
func benchRGBA(width, height int, pixel color.RGBA) *image.RGBA {
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			pic.SetRGBA(x, y, pixel)
		}
	}
	return pic
}

// BenchmarkStroke paints a 200-point stroke onto a 612 by 792 pixmap.
func BenchmarkStroke(b *testing.B) {
	pixmap := NewPixmap(benchPageWidth, benchPageHeight)
	pts := benchPoints()
	b.SetBytes(int64(len(pts)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.Stroke(pts, 1, 0, 0, 0)
	}
}

// BenchmarkFill fills a 200 by 200 square on a 612 by 792 pixmap.
func BenchmarkFill(b *testing.B) {
	pixmap := NewPixmap(benchPageWidth, benchPageHeight)
	pts := benchSquare()
	b.SetBytes(int64(len(pts)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.Fill(pts, 0, 0, 0, false)
	}
}

// BenchmarkDrawImage stamps an image at 1:1 and at four times the size.
func BenchmarkDrawImage(b *testing.B) {
	cases := []struct {
		name   string
		side   int
		scale  float64
		canvas int
	}{
		{name: "1to1", side: 200, scale: 1, canvas: 200},
		{name: "scaled", side: 100, scale: 4, canvas: 400},
	}
	for _, testCase := range cases {
		b.Run(testCase.name, func(b *testing.B) {
			pixmap := NewPixmap(testCase.canvas, testCase.canvas)
			pic := benchRGBA(testCase.side, testCase.side, color.RGBA{R: 200, G: 100, B: 50, A: 255})
			b.SetBytes(int64(testCase.canvas * testCase.canvas))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				pixmap.DrawImage(pic, Identity(), testCase.scale)
			}
		})
	}
}

// benchStar builds a closed star polygon with the given number of points,
// centred on a benchFillSide page. Fill costs W*H*N, so the point count is the
// variable this file scales.
func benchStar(points int) []Point {
	pts := make([]Point, 0, points)
	centre := float64(benchFillSide) / 2
	outer, inner := centre*0.95, centre*0.45
	for i := range points {
		radius := outer
		if i%2 == 1 {
			radius = inner
		}
		angle := 2 * math.Pi * float64(i) / float64(points)
		pt := Point{
			X:    centre + radius*math.Cos(angle),
			Y:    centre + radius*math.Sin(angle),
			Move: i == 0,
		}
		pts = append(pts, pt)
	}
	return pts
}

// benchSquares builds count separate four point squares on a benchFillSide
// page, so subpaths returns count subpaths and inside walks all of them for
// every pixel.
func benchSquares(count int) []Point {
	pts := make([]Point, 0, count*4)
	for i := range count {
		originX := float64((i % 8) * 24)
		originY := float64((i / 8) * 24)
		pts = append(pts,
			Point{X: originX, Y: originY, Move: true},
			Point{X: originX + 20, Y: originY},
			Point{X: originX + 20, Y: originY + 20},
			Point{X: originX, Y: originY + 20},
		)
	}
	return pts
}

// benchClip returns one clip path that covers the middle of the fill page, so
// a mark on the full page has pixels both inside and outside the clip.
func benchClip() []Clip {
	return []Clip{{
		Pts: []Point{
			{X: 50, Y: 50, Move: true},
			{X: 150, Y: 50},
			{X: 150, Y: 150},
			{X: 50, Y: 150},
		},
	}}
}

// benchGlyphMask builds a square coverage mask with one constant alpha value.
func benchGlyphMask(alpha uint8) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, benchGlyphSide, benchGlyphSide))
	for y := range benchGlyphSide {
		for x := range benchGlyphSide {
			mask.SetAlpha(x, y, color.Alpha{A: alpha})
		}
	}
	return mask
}

// BenchmarkFillComplexity fills a star of 4, 32, and 128 points on a 200 by
// 200 page. Pixmap.Fill walks every pixel and calls inside for each one, and
// inside walks every segment, so the cost is W*H*N and the three lines are the
// slope. The existing BenchmarkFill only reaches N=4.
func BenchmarkFillComplexity(b *testing.B) {
	for _, points := range []int{4, 32, 128} {
		b.Run(strconv.Itoa(points), func(b *testing.B) {
			pixmap := NewPixmap(benchFillSide, benchFillSide)
			pts := benchStar(points)
			b.SetBytes(int64(benchFillSide * benchFillSide))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				pixmap.Fill(pts, 0, 0, 0, false)
			}
		})
	}
}

// BenchmarkFillSubpaths fills 32 separate squares, so subpaths returns 32
// subpaths and inside tests all of them for every pixel.
func BenchmarkFillSubpaths(b *testing.B) {
	pixmap := NewPixmap(benchFillSide, benchFillSide)
	pts := benchSquares(32)
	b.SetBytes(int64(benchFillSide * benchFillSide))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.Fill(pts, 0, 0, 0, false)
	}
}

// BenchmarkStrokeClipped strokes a 200 point polyline through one clip path.
// Each call allocates a rectSnapshot of the path bounds at clip.go:152 and
// then walks that rectangle again in restoreOutside.
func BenchmarkStrokeClipped(b *testing.B) {
	pixmap := NewPixmap(benchPageWidth, benchPageHeight)
	pts := benchPoints()
	clips := benchClip()
	b.SetBytes(int64(len(pts)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.StrokeClipped(clips, pts, 1, 0, 0, 0)
	}
}

// BenchmarkFillClipped fills the 200 by 200 square through one clip path.
func BenchmarkFillClipped(b *testing.B) {
	pixmap := NewPixmap(benchPageWidth, benchPageHeight)
	pts := benchSquare()
	clips := benchClip()
	b.SetBytes(int64(benchPageWidth * benchPageHeight))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.FillClipped(clips, pts, 0, 0, 0, false)
	}
}

// BenchmarkCompositeGroupClipped composites a group under one clip path. The
// clipped form snapshots the whole page at clip.go:78 rather than the group
// bounds, and the group-only sub-benchmark is the same composite with no
// snapshot.
func BenchmarkCompositeGroupClipped(b *testing.B) {
	group := NewGroupPixmap(benchFillSide, benchFillSide)
	clips := benchClip()
	b.Run("group-only", func(b *testing.B) {
		pixmap := NewPixmap(benchFillSide, benchFillSide)
		b.SetBytes(int64(benchFillSide * benchFillSide))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			pixmap.CompositeGroup(group, 1, BlendNormal)
		}
	})
	b.Run("clipped", func(b *testing.B) {
		pixmap := NewPixmap(benchFillSide, benchFillSide)
		b.SetBytes(int64(benchFillSide * benchFillSide))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			pixmap.CompositeGroupClipped(clips, group, 1, BlendNormal)
		}
	})
}

// BenchmarkDrawGlyph blends a coverage mask at full, half, and zero coverage.
// The zero case returns at pixmap.go:259 before the blend, so it is the cost
// of walking the mask with nothing to write.
func BenchmarkDrawGlyph(b *testing.B) {
	for _, testCase := range []struct {
		name  string
		alpha uint8
	}{
		{name: "full", alpha: 255},
		{name: "half", alpha: 128},
		{name: "zero", alpha: 0},
	} {
		b.Run(testCase.name, func(b *testing.B) {
			pixmap := NewPixmap(benchPageWidth, benchPageHeight)
			mask := benchGlyphMask(testCase.alpha)
			b.SetBytes(int64(benchGlyphSide * benchGlyphSide))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				pixmap.DrawGlyph(mask, 100, 100, 0, 0, 0)
			}
		})
	}
}

// BenchmarkShowPage copies the full pixel plane and re-whitens the pixel and
// alpha planes. Every call also retains a page copy by design, so the retained
// pages are dropped inside the loop. Without that a -count=3 run would hold
// gigabytes, and the copy is the cost being measured.
func BenchmarkShowPage(b *testing.B) {
	pixmap := NewPixmap(benchPageWidth, benchPageHeight)
	b.SetBytes(int64(benchPageWidth * benchPageHeight * 3))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pixmap.ShowPage()
		pixmap.pages = pixmap.pages[:0]
	}
}
