package pdf

import "testing"

// TestPageResourcesDirect proves that a page's own /Resources resolves,
// including the /XObject subdictionary, and that the nearest /Resources wins
// over an ancestor's. A bad index is rangecheck.
func TestPageResourcesDirect(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 " +
		"/Resources << /XObject << /Im0 5 0 R >> >> >>")
	doc.object("<< /Type /Page /Parent 2 0 R " +
		"/Resources << /XObject << /Im0 4 0 R >> >> >>")
	doc.object(imageStream(t, 2, 2, "/DeviceRGB", "/FlateDecode", flateRaw(t, rgbPixels())))
	doc.object(imageStream(t, 1, 1, "/DeviceRGB", "/FlateDecode", flateRaw(t, []byte{1, 2, 3})))
	file := mustOpen(t, doc.classic(""))
	res, err := file.PageResources(0)
	if err != nil {
		t.Fatal(err)
	}
	checkResourceImage(t, res, "Im0", 2)
	_, err = file.PageResources(1)
	wantJob(t, err, opRaster, errRange)
}

// TestPageResourcesInherited proves that /Resources on a /Pages ancestor
// reaches the page leaf through two /Pages levels. The resources value, the
// /XObject subdictionary, and the image are all indirect references.
func TestPageResourcesInherited(t *testing.T) {
	t.Parallel()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 /Resources 5 0 R >>")
	doc.object("<< /Type /Pages /Kids [4 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 3 0 R >>")
	doc.object("<< /XObject 6 0 R >>")
	doc.object("<< /Im0 7 0 R >>")
	doc.object(imageStream(t, 2, 2, "/DeviceRGB", "/FlateDecode", flateRaw(t, rgbPixels())))
	file := mustOpen(t, doc.classic(""))
	res, err := file.PageResources(0)
	if err != nil {
		t.Fatal(err)
	}
	checkResourceImage(t, res, "Im0", 2)
}

func checkResourceImage(t *testing.T, res Resources, name string, width int) {
	t.Helper()
	val, ok := res.XObjects[name]
	if !ok {
		t.Fatalf("xobject %q is missing", name)
	}
	if !hasImageSubtype(val) {
		t.Fatalf("xobject %q kind %v is not an image", name, val.Kind)
	}
	got, ok := val.IntEntry(keyWidth)
	if !ok || got != width {
		t.Fatalf("xobject %q width = %d ok %v, want %d", name, got, ok, width)
	}
}
