package spectreps_test

import (
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationCanceledJobs locks the canceled-context path of the three
// jobs that had none: ExtractText, ImagePDF, and ImagePDFColor.
func TestValidationCanceledJobs(t *testing.T) {
	in := newInst(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	page := validationPage()

	t.Run("ExtractText", func(t *testing.T) {
		text, err := in.ExtractText(ctx, nil, 0)
		validationCanceledError(t, err)
		if text != "" {
			t.Fatalf("ExtractText() text = %q, want empty", text)
		}
	})
	t.Run("ImagePDF", func(t *testing.T) {
		out, err := in.ImagePDF(ctx, []spectreps.PageImage{page}, 72)
		validationCanceledError(t, err)
		if out != nil {
			t.Fatalf("ImagePDF() bytes = %#v, want nil", out)
		}
	})
	t.Run("ImagePDFColor", func(t *testing.T) {
		out, err := in.ImagePDFColor(ctx, []spectreps.PageImage{page}, 72, spectreps.ImageColorCMYK)
		validationCanceledError(t, err)
		if out != nil {
			t.Fatalf("ImagePDFColor() bytes = %#v, want nil", out)
		}
	})
}

// TestValidationNilContextJobs locks the nil-context panic of the same three
// jobs. The package panics with "spectreps: nil context".
func TestValidationNilContextJobs(t *testing.T) {
	in := newInst(t)
	page := validationPage()
	pages := []spectreps.PageImage{page}
	imageColor := spectreps.ImageColorGray

	requirePanic(t, func() {
		_, err := in.ExtractText(nil, nil, 0) //nolint:staticcheck // nil context is the case under test
		if err != nil {
			t.Errorf("ExtractText() error = %v", err)
		}
	})
	requirePanic(t, func() {
		_, err := in.ImagePDF(nil, pages, 72) //nolint:staticcheck // nil context is the case under test
		if err != nil {
			t.Errorf("ImagePDF() error = %v", err)
		}
	})
	requirePanic(t, func() {
		_, err := in.ImagePDFColor(nil, pages, 72, imageColor) //nolint:staticcheck // nil context is the case under test
		if err != nil {
			t.Errorf("ImagePDFColor() error = %v", err)
		}
	})
}

func validationPage() spectreps.PageImage {
	return spectreps.PageImage{
		Width:  1,
		Height: 1,
		Stride: 3,
		Pixels: []byte{255, 255, 255},
	}
}

func validationCanceledError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
