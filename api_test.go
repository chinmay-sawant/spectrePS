package spectreps_test

import (
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS"
)

func TestVersion(t *testing.T) {
	if got := spectreps.Version(); got != "0.0.1" {
		t.Fatalf("Version() = %q, want %q", got, "0.0.1")
	}
}

func TestNewClose(t *testing.T) {
	in, err := spectreps.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if in == nil {
		t.Fatalf("New() instance = nil")
	}
	if err := in.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := in.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestJobErrorText(t *testing.T) {
	withFile := spectreps.JobError{
		Op:       "add",
		Msg:      "stackunderflow",
		Filename: "box.ps",
		Line:     3,
		Column:   5,
	}
	const wantWithFile = "Error: /stackunderflow in add at box.ps:3:5"
	if got := withFile.Error(); got != wantWithFile {
		t.Fatalf("JobError.Error() = %q, want %q", got, wantWithFile)
	}

	noFile := spectreps.JobError{Op: "add", Msg: "stackunderflow"}
	const wantNoFile = "Error: /stackunderflow in add"
	if got := noFile.Error(); got != wantNoFile {
		t.Fatalf("JobError.Error() = %q, want %q", got, wantNoFile)
	}

	var asError error = spectreps.JobError{
		Op:       "add",
		Msg:      "stackunderflow",
		Filename: "box.ps",
		Line:     3,
		Column:   5,
	}
	if asError.Error() != wantWithFile {
		t.Fatalf("error.Error() = %q, want %q", asError.Error(), wantWithFile)
	}
}

func TestNotImplemented(t *testing.T) {
	in, err := spectreps.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if in == nil {
		t.Fatalf("New() instance = nil")
	}

	src := []byte("%!PS")
	var opt spectreps.RunOptions
	rewrite := spectreps.DefaultRewriteOptions()

	t.Run("background", func(t *testing.T) {
		pages, err := in.RunPostScript(context.Background(), src, opt)
		if !errors.Is(err, spectreps.ErrNotImplemented) {
			t.Fatalf("RunPostScript() error = %v, want ErrNotImplemented", err)
		}
		if pages != nil {
			t.Fatalf("RunPostScript() pages = %#v, want nil", pages)
		}

		doc, err := in.OpenPDF(context.Background(), src)
		if !errors.Is(err, spectreps.ErrNotImplemented) {
			t.Fatalf("OpenPDF() error = %v, want ErrNotImplemented", err)
		}
		if doc != nil {
			t.Fatalf("OpenPDF() document = %#v, want nil", doc)
		}

		img, err := in.RasterizePage(context.Background(), nil, 0, opt)
		if !errors.Is(err, spectreps.ErrNotImplemented) {
			t.Fatalf("RasterizePage() error = %v, want ErrNotImplemented", err)
		}
		requireZeroPageImage(t, img)

		out, err := in.RewritePDF(context.Background(), nil, rewrite)
		if !errors.Is(err, spectreps.ErrNotImplemented) {
			t.Fatalf("RewritePDF() error = %v, want ErrNotImplemented", err)
		}
		if out != nil {
			t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		pages, err := in.RunPostScript(ctx, src, opt)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunPostScript() error = %v, want context.Canceled", err)
		}
		if pages != nil {
			t.Fatalf("RunPostScript() pages = %#v, want nil", pages)
		}

		doc, err := in.OpenPDF(ctx, src)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("OpenPDF() error = %v, want context.Canceled", err)
		}
		if doc != nil {
			t.Fatalf("OpenPDF() document = %#v, want nil", doc)
		}

		img, err := in.RasterizePage(ctx, nil, 0, opt)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RasterizePage() error = %v, want context.Canceled", err)
		}
		requireZeroPageImage(t, img)

		out, err := in.RewritePDF(ctx, nil, rewrite)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RewritePDF() error = %v, want context.Canceled", err)
		}
		if out != nil {
			t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
		}
	})

	t.Run("nil context", func(t *testing.T) {
		const want = "spectreps: nil context"
		t.Run("RunPostScript", func(t *testing.T) {
			requirePanic(t, want, func() {
				in.RunPostScript(nil, src, opt)
			})
		})
		t.Run("OpenPDF", func(t *testing.T) {
			requirePanic(t, want, func() {
				in.OpenPDF(nil, src)
			})
		})
		t.Run("RasterizePage", func(t *testing.T) {
			requirePanic(t, want, func() {
				in.RasterizePage(nil, nil, 0, opt)
			})
		})
		t.Run("RewritePDF", func(t *testing.T) {
			requirePanic(t, want, func() {
				in.RewritePDF(nil, nil, rewrite)
			})
		})
	})

	t.Run("CompareRaster", func(t *testing.T) {
		requirePanicIs(t, spectreps.ErrNotImplemented, func() {
			spectreps.CompareRaster(spectreps.PageImage{}, spectreps.PageImage{})
		})
	})
}

func TestCompareFiles(t *testing.T) {
	tests := []struct {
		name   string
		a, b   []byte
		equal  bool
		offset int64
		reason string
	}{
		{name: "nil", a: nil, b: nil, equal: true, offset: -1, reason: ""},
		{name: "empty", a: []byte{}, b: []byte{}, equal: true, offset: -1, reason: ""},
		{name: "equal", a: []byte("abc"), b: []byte("abc"), equal: true, offset: -1, reason: ""},
		{name: "byte", a: []byte("abc"), b: []byte("axc"), equal: false, offset: 1, reason: "byte"},
		{name: "prefix", a: []byte("abc"), b: []byte("ab"), equal: false, offset: 2, reason: "length"},
		{name: "longer", a: []byte("ab"), b: []byte("abc"), equal: false, offset: 2, reason: "length"},
		{name: "first", a: []byte("a"), b: []byte("b"), equal: false, offset: 0, reason: "byte"},
		{name: "byte before length", a: []byte("ab"), b: []byte("x"), equal: false, offset: 0, reason: "byte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spectreps.CompareFiles(tt.a, tt.b)
			want := spectreps.CompareResult{Equal: tt.equal, Offset: tt.offset, Reason: tt.reason}
			if got != want {
				t.Fatalf("CompareFiles(%#v, %#v) = %+v, want %+v", tt.a, tt.b, got, want)
			}
		})
	}
}

func TestDefaultRewriteOptions(t *testing.T) {
	opt := spectreps.DefaultRewriteOptions()
	if !opt.CompressStreams {
		t.Fatalf("DefaultRewriteOptions().CompressStreams = false, want true")
	}
	var zero spectreps.RewriteOptions
	if zero.CompressStreams {
		t.Fatalf("zero RewriteOptions.CompressStreams = true, want false")
	}
}

func requireZeroPageImage(t *testing.T, img spectreps.PageImage) {
	t.Helper()
	if img.Width != 0 || img.Height != 0 || img.Stride != 0 || img.Pixels != nil {
		t.Fatalf("PageImage = %#v, want zero value", img)
	}
}

func requirePanic(t *testing.T, want string, fn func()) {
	t.Helper()
	recovered, panicked := catchPanic(fn)
	if !panicked {
		t.Fatalf("no panic, want %q", want)
	}
	got, ok := recovered.(string)
	if !ok || got != want {
		t.Fatalf("panic = %#v, want %q", recovered, want)
	}
}

func requirePanicIs(t *testing.T, target error, fn func()) {
	t.Helper()
	recovered, panicked := catchPanic(fn)
	if !panicked {
		t.Fatalf("no panic, want %v", target)
	}
	err, ok := recovered.(error)
	if !ok || !errors.Is(err, target) {
		t.Fatalf("panic = %#v, want errors.Is %v", recovered, target)
	}
}

func catchPanic(fn func()) (any, bool) {
	var recovered any
	panicked := false
	func() {
		defer func() {
			recovered = recover()
			if recovered != nil {
				panicked = true
			}
		}()
		fn()
	}()
	return recovered, panicked
}
