package pdfout

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestValidationLevelOverridesErrors locks the LevelOverrides error surface:
// a nil file, the level bounds, a canceled context, a nil context, and the
// level 2 pass-through cases for a two-name filter chain and a null /SMask.
func TestValidationLevelOverridesErrors(t *testing.T) {
	t.Run("nil file", func(t *testing.T) {
		checkLevelRejected(t, nil, 2, errType)
	})
	t.Run("level 0", func(t *testing.T) {
		checkLevelRejected(t, levelTextPDF(t), 0, errRange)
	})
	t.Run("level 6", func(t *testing.T) {
		checkLevelRejected(t, levelTextPDF(t), 6, errRange)
	})
	t.Run("canceled context", checkLevelCanceled)
	t.Run("nil context", checkLevelNilContext)
	t.Run("two name chain", checkLevelTwoNameChain)
	t.Run("null smask", checkLevelNullSMask)
}

// checkLevelRejected requires one rejected level to return the named error and
// no overrides.
func checkLevelRejected(t *testing.T, file *pdf.File, level int, name string) {
	t.Helper()
	overrides, err := LevelOverrides(t.Context(), file, level)
	checkLevelError(t, err, opLevel, name)
	if overrides != nil {
		t.Fatalf("overrides = %v, want nil", overrides)
	}
}

// checkLevelCanceled proves a canceled context returns context.Canceled and no
// overrides.
func checkLevelCanceled(t *testing.T) {
	t.Helper()
	file := levelTextPDF(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	overrides, err := LevelOverrides(ctx, file, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if overrides != nil {
		t.Fatalf("overrides = %v, want nil", overrides)
	}
}

// checkLevelNilContext proves a nil context panics with the documented value.
func checkLevelNilContext(t *testing.T) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered != panicNilContext {
			t.Fatalf("panic = %v, want %s", recovered, panicNilContext)
		}
	}()
	file := levelTextPDF(t)
	_, _ = LevelOverrides(nil, file, 2) //nolint:staticcheck // nil context is the case under test
	t.Fatal("LevelOverrides returned")
}

// checkLevelTwoNameChain proves level 2 leaves an image with a two-name filter
// chain alone and copies the chain back out as an array.
func checkLevelTwoNameChain(t *testing.T) {
	t.Helper()
	body := levelStreamBody(
		"/Type /XObject /Subtype /Image /Width 4 /Height 4 "+
			"/ColorSpace /DeviceRGB /BitsPerComponent 8 "+
			"/Filter [/FlateDecode /DCTDecode]",
		flateLevelBytes(t, levelSamples()),
	)
	file := levelImagePDF(t, body)
	overrides, err := LevelOverrides(t.Context(), file, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := overrides[levelImageNum]; ok {
		t.Fatal("level 2 rewrote an image with a two-name filter chain")
	}
	copiedBytes := mustCopy(t, file, CopyOptions{Overrides: overrides})
	if !bytes.Contains(copiedBytes, []byte("/Filter [/FlateDecode /DCTDecode]")) {
		t.Fatal("the copied chain lost its filter array")
	}
	copied := mustOpenPDF(t, copiedBytes)
	val, ok, err := copied.ObjectValue(levelImageNum)
	if err != nil || !ok {
		t.Fatalf("image object ok %v err %v", ok, err)
	}
	name, found := val.NameEntry(keyFilter)
	if found || name != "" {
		t.Fatalf("copied chain named a single filter %q", name)
	}
}

// checkLevelNullSMask proves level 2 rewrites an image with a null /SMask,
// keeps the entry, and leaves the pixels decodable.
func checkLevelNullSMask(t *testing.T) {
	t.Helper()
	body := levelStreamBody(
		"/Type /XObject /Subtype /Image /Width 4 /Height 4 "+
			"/ColorSpace /DeviceRGB /BitsPerComponent 8 /SMask null",
		levelSamples(),
	)
	file := levelImagePDF(t, body)
	overrides, err := LevelOverrides(t.Context(), file, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := overrides[levelImageNum]; !ok {
		t.Fatal("level 2 left the raw image with a null /SMask alone")
	}
	reopened := mustOpenPDF(t, mustCopy(t, file, CopyOptions{Overrides: overrides}))
	checkLevelFilter(t, reopened, levelImageNum, filterFlate)
	val, ok, err := reopened.ObjectValue(levelImageNum)
	if err != nil || !ok {
		t.Fatalf("image object ok %v err %v", ok, err)
	}
	if _, found := val.ValueEntry(keySMask); !found {
		t.Fatal("level 2 dropped the null /SMask entry")
	}
	pic, err := reopened.DecodeImage(levelImageNum)
	if err != nil {
		t.Fatalf("DecodeImage() = %v", err)
	}
	checkLevelPixels(t, pic, levelSamples())
}

// checkLevelError requires one *pdf.Error with the given operator and name.
func checkLevelError(t *testing.T, err error, op, name string) {
	t.Helper()
	var job *pdf.Error
	if !errors.As(err, &job) || job.Op != op || job.Name != name {
		t.Fatalf("error = %v, want %s in %s", err, name, op)
	}
}
