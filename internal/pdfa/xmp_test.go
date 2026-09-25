package pdfa

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

func TestXMPPacket(t *testing.T) {
	base := XMP(Mode4)
	conformance := XMP(Mode4F)
	if base == nil || conformance == nil {
		t.Fatal("Mode4 or Mode4F has no packet")
	}
	if XMP(ModeNone) != nil {
		t.Fatal("ModeNone has a packet")
	}
	checkXMPPacket(t, "base", base)
	checkXMPPacket(t, "4f", conformance)
	if bytes.Contains(base, []byte("pdfaid:conformance")) {
		t.Fatal("base packet carries a conformance letter")
	}
	if !bytes.Contains(conformance, []byte("<pdfaid:conformance>F</pdfaid:conformance>")) {
		t.Fatal("4f packet is missing the F conformance letter")
	}
	if !bytes.Equal(base, XMP(Mode4)) {
		t.Fatal("two calls differ")
	}
}

func checkXMPPacket(t *testing.T, name string, packet []byte) {
	t.Helper()
	if !utf8.Valid(packet) {
		t.Fatalf("%s packet is not UTF-8", name)
	}
	needles := []string{
		"<?xpacket begin=\"\uFEFF\"",
		"<?xpacket end=\"w\"?>",
		"<pdfaid:part>4</pdfaid:part>",
		"<pdfaid:rev>2020</pdfaid:rev>",
	}
	for _, needle := range needles {
		if !bytes.Contains(packet, []byte(needle)) {
			t.Fatalf("%s packet is missing %q", name, needle)
		}
	}
	dates := []string{"xmp:CreateDate", "xmp:ModifyDate", "xmp:MetadataDate", "CreationDate", "ModDate"}
	for _, date := range dates {
		if bytes.Contains(packet, []byte(date)) {
			t.Fatalf("%s packet carries %s", name, date)
		}
	}
}
