package ps

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// benchStrokeProgram paints 2000 horizontal strokes on a 200 by 200 point page.
const benchStrokeProgram = "0 1 1999 { /y exch def 0 y moveto 200 y lineto 1 setlinewidth stroke } for"

// BenchmarkStrokeProgram runs the synthetic stroke program through a fresh
// interpreter and pixmap, the same setup RunPostScript uses.
func BenchmarkStrokeProgram(b *testing.B) {
	src := []byte(benchStrokeProgram)
	ctx := b.Context()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		interp := NewInterp()
		interp.UsePixmap(graphics.NewPixmap(200, 200), 1)
		if err := interp.Run(ctx, src); err != nil {
			b.Fatal(err)
		}
	}
}
