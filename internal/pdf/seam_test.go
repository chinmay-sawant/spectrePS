package pdf

import (
	"image"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// runLog records TextRun events in call order.
type runLog struct {
	runs []TextRun
}

func (log *runLog) TextRun(run TextRun) {
	log.runs = append(log.runs, run)
}

// TestTextRunEvents proves one event per show operator with the shown bytes and
// the current text state, with TJ numbers omitted.
func TestTextRunEvents(t *testing.T) {
	run := textRunner(t)
	log := &runLog{}
	run.runs = log
	src := "BT /F1 12 Tf 1 Tc 2 Tw 50 Tz 3 Ts 10 20 Td " +
		"(AB) Tj [ (C) -100 (D) ] TJ (E) ' 1 2 (F) \" ET"
	playContent(t, run, src)
	if len(log.runs) != 4 {
		t.Fatalf("runs = %d, want 4", len(log.runs))
	}
	checkTextRunBytes(t, log.runs)
	checkTextRunSpacing(t, log.runs)
	checkTextRunState(t, log.runs)
	t.Run("typed nil", func(t *testing.T) {
		typed := textRunner(t)
		var sink *runLog
		typed.runs = sink
		playContent(t, typed, "BT /F1 12 Tf (AB) Tj ET")
	})
}

// checkTextRunBytes proves the shown bytes per operator, with TJ numbers gone.
func checkTextRunBytes(t *testing.T, runs []TextRun) {
	t.Helper()
	want := []string{"AB", "CD", "E", "F"}
	for idx, got := range runs {
		if string(got.Bytes) != want[idx] {
			t.Fatalf("run %d bytes = %q, want %q", idx, got.Bytes, want[idx])
		}
		if got.FontName != "F1" || got.Size != 12 {
			t.Fatalf("run %d font = %q %v", idx, got.FontName, got.Size)
		}
	}
}

// checkTextRunSpacing proves the spacing values the show operator ran with.
func checkTextRunSpacing(t *testing.T, runs []TextRun) {
	t.Helper()
	for idx := range 3 {
		if runs[idx].CharSpacing != 1 || runs[idx].WordSpacing != 2 {
			t.Fatalf("run %d spacing = %+v", idx, runs[idx])
		}
	}
	if runs[3].CharSpacing != 2 || runs[3].WordSpacing != 1 {
		t.Fatalf("run 3 spacing = %+v", runs[3])
	}
}

// checkTextRunState proves the size, rise, scale, and matrices in each event.
func checkTextRunState(t *testing.T, runs []TextRun) {
	t.Helper()
	for idx, got := range runs {
		if got.HScale != 0.5 || got.Rise != 3 {
			t.Fatalf("run %d state = %+v", idx, got)
		}
	}
	first := runs[0].TextMatrix
	if first.E != 10 || first.F != 20 || first != runs[0].LineMatrix {
		t.Fatalf("run 0 matrices = %+v %+v", first, runs[0].LineMatrix)
	}
}

// imageLog records ImageName events, and implements graphics.Marker so it can
// stand in for a recorder.
type imageLog struct {
	names []string
	dicts []Value
}

func (log *imageLog) ImageName(name string, dict Value) {
	log.names = append(log.names, name)
	log.dicts = append(log.dicts, dict)
}

func (log *imageLog) Stroke(_ []graphics.Point, _, _, _, _ float64) {}

func (log *imageLog) Fill(_ []graphics.Point, _, _, _ float64, _ bool) {}

func (log *imageLog) DrawImage(_ image.Image, _ graphics.Matrix, _ float64) {}

// TestPaintImageName proves Do names an image XObject before decode through
// the optional ImageNameMarker seam.
func TestPaintImageName(t *testing.T) {
	t.Run("named", func(t *testing.T) {
		file := doPage(t, "/Im0 Do", "<< /XObject << /Im0 5 0 R >> >>", rgbImageBody(t))
		log := imageNameLog(t, file)
		if len(log.names) != 1 || log.names[0] != "Im0" {
			t.Fatalf("names = %v", log.names)
		}
		if len(log.dicts) != 1 || !hasImageSubtype(log.dicts[0]) {
			t.Fatalf("dict = %+v", log.dicts)
		}
	})
	t.Run("before decode", func(t *testing.T) {
		file := doPage(t, "/Im0 Do", "<< /XObject << /Im0 5 0 R >> >>",
			maskedImageBody(t),
			imageStream(t, 2, 2, "/DeviceGray", "/FlateDecode", flateRaw(t, grayPixels())))
		log := imageNameLog(t, file)
		if len(log.names) != 1 || log.names[0] != "Im0" {
			t.Fatalf("names = %v", log.names)
		}
	})
}

// imageNameLog paints one page onto the image log and returns it. A page that
// still fails is checked after the event count.
func imageNameLog(t *testing.T, file *File) *imageLog {
	t.Helper()
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := file.PageResources(0)
	if err != nil {
		t.Fatal(err)
	}
	log := &imageLog{}
	err = PaintWith(t.Context(), content, log, 1, PaintOptions{Resources: res})
	if err != nil {
		wantJobErr(t, err, "Do", nameUndefined)
	}
	return log
}
