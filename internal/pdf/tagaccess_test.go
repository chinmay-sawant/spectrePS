package pdf

import "testing"

// TestPageObjectNums proves the page object numbers come back in PageCount
// order and name the page dictionaries.
func TestPageObjectNums(t *testing.T) {
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
		"/Resources << >> >>")
	doc.object(streamBody("", []byte("0 0 m 10 0 l S")))
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 6 0 R " +
		"/Resources << >> >>")
	doc.object(streamBody("", []byte("0 0 m 10 0 l S")))
	file := mustOpen(t, doc.classic())
	nums := file.PageObjectNums()
	if len(nums) != file.PageCount() || len(nums) != 2 {
		t.Fatalf("PageObjectNums() = %v, PageCount() = %d", nums, file.PageCount())
	}
	if nums[0] != 3 || nums[1] != 5 {
		t.Fatalf("PageObjectNums() = %v", nums)
	}
	for _, num := range nums {
		val, ok, err := file.ObjectValue(num)
		if err != nil || !ok {
			t.Fatalf("object %d = %v %v", num, ok, err)
		}
		if name, _ := val.NameEntry("Type"); name != "Page" {
			t.Fatalf("object %d type = %q", num, name)
		}
	}
}

// TestPageFontDict proves the font dictionary accessor pairs with the loaded
// resource and TaggedFontOK reads it.
func TestPageFontDict(t *testing.T) {
	doc := newTaggedDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(streamBody("", []byte("0 0 m 10 0 l S")))
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica " +
		"/Encoding /WinAnsiEncoding >>")
	file := mustOpen(t, doc.classic())
	dict, ok, err := file.PageFontDict(0, "F1")
	if err != nil || !ok {
		t.Fatalf("PageFontDict(F1) = %v %v", ok, err)
	}
	if !TaggedFontOK(dict) {
		t.Fatalf("TaggedFontOK(%v) = false", dict)
	}
	if _, ok, err := file.PageFontDict(0, "F2"); err != nil || ok {
		t.Fatalf("PageFontDict(F2) = %v %v", ok, err)
	}
	if _, _, err := file.PageFontDict(4, "F1"); err == nil {
		t.Fatal("PageFontDict(4) accepted a bad page")
	}
}
