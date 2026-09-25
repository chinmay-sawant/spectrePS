package pdfa

import (
	"bytes"
	"testing"
)

func TestOutputIntent(t *testing.T) {
	bodies := ExtraObjects(Mode4, 10)
	if len(bodies) != 3 {
		t.Fatalf("bodies %d, want 3", len(bodies))
	}
	metadata, profile, intent := bodies[0], bodies[1], bodies[2]
	if !bytes.Contains(metadata, []byte("/Type /Metadata")) {
		t.Fatal("metadata object is missing /Type /Metadata")
	}
	if !bytes.Contains(metadata, []byte("<pdfaid:part>4</pdfaid:part>")) {
		t.Fatal("metadata object is missing the packet")
	}
	if !bytes.Contains(profile, []byte("/N 3")) {
		t.Fatal("ICC stream is missing /N 3")
	}
	if !bytes.Contains(intent, []byte("/S /GTS_PDFA1")) {
		t.Fatal("intent is missing /S /GTS_PDFA1")
	}
	if !bytes.Contains(intent, []byte("/DestOutputProfile 11 0 R")) {
		t.Fatal("intent is missing /DestOutputProfile on the ICC stream")
	}
	if bytes.Contains(intent, []byte("DestOutputProfileRef")) {
		t.Fatal("intent carries /DestOutputProfileRef")
	}
	checkOutputIntentMode4F(t)
	if ExtraObjects(ModeNone, 1) != nil {
		t.Fatal("ModeNone has extra objects")
	}
	if ExtraObjects(Mode4, 0) != nil {
		t.Fatal("object number 0 has extra objects")
	}
}

func checkOutputIntentMode4F(t *testing.T) {
	t.Helper()
	bodies := ExtraObjects(Mode4F, 1)
	if len(bodies) != 3 {
		t.Fatalf("4f bodies %d, want 3", len(bodies))
	}
	if !bytes.Contains(bodies[0], []byte("<pdfaid:conformance>F</pdfaid:conformance>")) {
		t.Fatal("4f metadata is missing the F conformance letter")
	}
	if !bytes.Contains(bodies[2], []byte("/DestOutputProfile 2 0 R")) {
		t.Fatal("4f intent does not point at object 2")
	}
}
