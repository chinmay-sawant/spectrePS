package tag

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestTagRecorder proves the recorder implements the marker and seam shapes
// and records events in stream order.
func TestTagRecorder(t *testing.T) {
	var _ graphics.Marker = (*Recorder)(nil)
	var _ MarkedContentSink = (*Recorder)(nil)
	var _ TextRunSink = (*Recorder)(nil)
	var _ ImageNameMarker = (*Recorder)(nil)

	rec := NewRecorder()
	rec.Stroke([]graphics.Point{{X: 0, Y: 0, Move: true}, {X: 10, Y: 0}}, 1, 0, 0, 0)
	rec.BeginMarkedContent("P", pdf.DictVal(map[string]pdf.Value{"MCID": pdf.IntVal(0)}), 0)
	rec.TextRun(TextRun{
		FontName: "F1", Size: 10,
		TextMatrix: graphics.Identity(), LineMatrix: graphics.Identity(),
		Rise: 0, CharSpacing: 0, WordSpacing: 0, HScale: 1, Bytes: []byte("A"),
	})
	rec.ImageName("Im0", pdf.DictVal(map[string]pdf.Value{"Subtype": pdf.NameVal("Image")}))
	rec.DrawImage(nil, graphics.Identity(), 1)
	rec.EndMarkedContent(0)
	rec.Fill([]graphics.Point{{X: 0, Y: 0, Move: true}, {X: 1, Y: 1}}, 0, 0, 0, false)

	if err := rec.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	want := []EventKind{
		EventStroke, EventBeginMarked, EventText,
		EventImage, EventEndMarked, EventFill,
	}
	events := rec.Events()
	if len(events) != len(want) {
		t.Fatalf("events = %d, want %d", len(events), len(want))
	}
	for index, kind := range want {
		if events[index].Kind != kind {
			t.Fatalf("event %d = %d, want %d", index, events[index].Kind, kind)
		}
	}
	if events[1].Depth != 0 || events[4].Depth != 0 {
		t.Fatalf("depths = %d, %d", events[1].Depth, events[4].Depth)
	}
	wantOrder(t, rec.Content(), []string{"S\n", "BDC\n", "Tj\n", "Do\n", "EMC\n", "f\n"})
}

// TestTagTextRun proves a run re-emits Tf, Tc, Tw, Tz, Ts, Tm, and Tj.
func TestTagTextRun(t *testing.T) {
	rec := NewRecorder()
	rec.TextRun(TextRun{
		FontName:    "F1",
		Size:        12,
		TextMatrix:  graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 10, F: 20},
		LineMatrix:  graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 10, F: 20},
		Rise:        2,
		CharSpacing: 1,
		WordSpacing: 3,
		HScale:      0.5,
		Bytes:       []byte("Hi"),
	})
	want := "BT\n/F1 12 Tf\n1 Tc\n3 Tw\n50 Tz\n2 Ts\n1 0 0 1 10 20 Tm\n<4869> Tj\nET\n"
	if got := string(rec.Content()); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

// TestTagImageName proves an image event re-emits Do from the recorded name,
// and a draw with no name refuses instead of dropping the image.
func TestTagImageName(t *testing.T) {
	t.Run("named", func(t *testing.T) {
		rec := NewRecorder()
		rec.ImageName("Im0", pdf.DictVal(map[string]pdf.Value{
			"Subtype": pdf.NameVal("Image"),
		}))
		rec.DrawImage(nil, graphics.Identity(), 1)
		if err := rec.Err(); err != nil {
			t.Fatalf("Err() = %v", err)
		}
		if got := string(rec.Content()); got != "/Im0 Do\n" {
			t.Fatalf("content = %q", got)
		}
	})
	t.Run("unnamed", func(t *testing.T) {
		rec := NewRecorder()
		rec.DrawImage(nil, graphics.Identity(), 1)
		var job *pdf.Error
		if !errors.As(rec.Err(), &job) || job.Op != opTag || job.Name != errImage {
			t.Fatalf("Err() = %v", rec.Err())
		}
	})
}

// wantOrder proves the needles appear in content in the given order.
func wantOrder(t *testing.T, content []byte, needles []string) {
	t.Helper()
	offset := 0
	for _, needle := range needles {
		index := bytes.Index(content[offset:], []byte(needle))
		if index < 0 {
			t.Fatalf("content %q lacks %q after byte %d", content, needle, offset)
		}
		offset += index + len(needle)
	}
}
