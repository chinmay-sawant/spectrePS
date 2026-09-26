package psout

import (
	"errors"
	"image"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestValidationPSOutImage locks the writer's image behavior: a content stream
// that runs an image name refuses with undefined in Do, and the recorder
// refuses a drawn image instead of dropping it.
func TestValidationPSOutImage(t *testing.T) {
	t.Run("emit name", func(t *testing.T) {
		got, err := Emit(t.Context(), []byte("/Im0 Do"))
		var job *pdf.Error
		if !errors.As(err, &job) || job.Op != opDo || job.Name != errUndefined {
			t.Fatalf("Emit() error = %v, want undefined in Do", err)
		}
		if got != nil {
			t.Fatalf("Emit() bytes = %#v, want nil", got)
		}
	})
	t.Run("recorder draw", func(t *testing.T) {
		rec := newRecorder()
		rec.Stroke([]graphics.Point{{X: 0, Y: 0, Move: true}, {X: 10, Y: 0}}, 1, 0, 0, 0)
		rec.DrawImage(image.NewRGBA(image.Rect(0, 0, 2, 2)), graphics.Matrix{}, 1)
		got, err := emitBytes(rec)
		var job *pdf.Error
		if !errors.As(err, &job) || job.Op != opDo || job.Name != errUndefined {
			t.Fatalf("emitBytes() error = %v, want undefined in Do", err)
		}
		if got != nil {
			t.Fatalf("emitBytes() bytes = %#v, want nil", got)
		}
	})
}
