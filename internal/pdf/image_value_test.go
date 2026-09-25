package pdf

import (
	"image"
	"testing"
)

// TestDecodeImageValue proves the value form of DecodeImage: a resolved
// object and its object number return the same pixels. Both the direct value
// and a value reached through an indirect reference decode.
func TestDecodeImageValue(t *testing.T) {
	t.Parallel()
	fixture := newImageFixture(t)
	file := mustOpen(t, fixture.src)
	checkDecodeValue(t, file, fixture.rgbNum, true)
	checkDecodeValue(t, file, fixture.grayNum, false)
	checkDecodeValueDCT(t, file, fixture.dctNum)
	checkDecodeValueUndefined(t)
}

func checkDecodeValue(t *testing.T, file *File, num int, rgb bool) {
	t.Helper()
	want, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	got, err := DecodeImageValue(val)
	if err != nil {
		t.Fatalf("direct value: %v", err)
	}
	checkSameImage(t, got, want)
	if rgb {
		checkRGBPixels(t, got, rgbPixels())
	} else {
		checkGrayPixels(t, got, grayPixels())
	}
	ref, err := file.deref(RefVal(num, 0))
	if err != nil {
		t.Fatal(err)
	}
	gotRef, err := DecodeImageValue(ref)
	if err != nil {
		t.Fatalf("indirect value: %v", err)
	}
	checkSameImage(t, gotRef, want)
}

func checkDecodeValueDCT(t *testing.T, file *File, num int) {
	t.Helper()
	want, err := file.DecodeImage(num)
	if err != nil {
		t.Fatal(err)
	}
	val, ok, err := file.ObjectValue(num)
	if err != nil || !ok {
		t.Fatalf("object %d ok %v err %v", num, ok, err)
	}
	got, err := DecodeImageValue(val)
	if err != nil {
		t.Fatal(err)
	}
	checkSameImage(t, got, want)
}

func checkDecodeValueUndefined(t *testing.T) {
	t.Helper()
	_, err := DecodeImageValue(NullVal())
	wantErr(t, err, opImage, errUndefined)
	dict := DictVal(map[string]Value{keySubtype: NameVal(nameImage)})
	_, err = DecodeImageValue(dict)
	wantErr(t, err, opImage, errUndefined)
}

func checkSameImage(t *testing.T, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v want %v", got.Bounds(), want.Bounds())
	}
	bounds := want.Bounds()
	for posY := bounds.Min.Y; posY < bounds.Max.Y; posY++ {
		for posX := bounds.Min.X; posX < bounds.Max.X; posX++ {
			gotR, gotG, gotB, gotA := got.At(posX, posY).RGBA()
			wantR, wantG, wantB, wantA := want.At(posX, posY).RGBA()
			if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
				t.Fatalf("pixel %d,%d got %v want %v", posX, posY, got.At(posX, posY), want.At(posX, posY))
			}
		}
	}
}
