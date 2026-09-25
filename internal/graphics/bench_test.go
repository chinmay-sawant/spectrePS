package graphics

import (
	"image"
	"image/color"
	"testing"
)

const (
	benchPageWidth  = 612
	benchPageHeight = 792
	benchPolyPoints = 200
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
