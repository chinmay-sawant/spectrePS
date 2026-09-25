package pdfa

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestICCProfile(t *testing.T) {
	profile := ICCProfile()
	if len(profile) < iccHeaderSize+4 {
		t.Fatalf("profile is %d bytes", len(profile))
	}
	checkICCHeader(t, profile)
	checkICCTags(t, profile)
	if !bytes.Equal(profile, ICCProfile()) {
		t.Fatal("two calls differ")
	}
}

func checkICCHeader(t *testing.T, profile []byte) {
	t.Helper()
	if size := int(binary.BigEndian.Uint32(profile[0:])); size != len(profile) {
		t.Fatalf("header size %d, want %d", size, len(profile))
	}
	if string(profile[12:16]) != iccClassMonitor {
		t.Fatalf("class %q", profile[12:16])
	}
	if string(profile[16:20]) != iccSpaceRGB {
		t.Fatalf("color space %q", profile[16:20])
	}
	if string(profile[20:24]) != iccSpaceXYZ {
		t.Fatalf("PCS %q", profile[20:24])
	}
	if string(profile[36:40]) != iccSignature {
		t.Fatalf("signature %q", profile[36:40])
	}
	if got := int32(binary.BigEndian.Uint32(profile[68:])); got != s15Fixed16(srgbWhite) {
		t.Fatalf("PCS illuminant X %d", got)
	}
}

func checkICCTags(t *testing.T, profile []byte) {
	t.Helper()
	for _, name := range []string{"bTRC", "bXYZ", "cprt", "desc", "gTRC", "gXYZ", "rTRC", "rXYZ", "wtpt"} {
		offset, _, ok := findICCTag(profile, name)
		if !ok {
			t.Fatalf("tag %s is missing or out of range", name)
		}
		if offset%4 != 0 {
			t.Fatalf("tag %s starts at %d, want a four-byte boundary", name, offset)
		}
	}
	checkICCCurve(t, profile, "rTRC")
	checkICCCurve(t, profile, "gTRC")
	checkICCCurve(t, profile, "bTRC")
	checkICCColorant(t, profile, "rXYZ")
	checkICCColorant(t, profile, "gXYZ")
	checkICCColorant(t, profile, "bXYZ")
}

func checkICCCurve(t *testing.T, profile []byte, name string) {
	t.Helper()
	offset, _, _ := findICCTag(profile, name)
	if string(profile[offset:offset+4]) != "curv" {
		t.Fatalf("tag %s type %q", name, profile[offset:offset+4])
	}
	if count := binary.BigEndian.Uint32(profile[offset+8:]); count != iccCurveSamples {
		t.Fatalf("tag %s sample count %d", name, count)
	}
}

func checkICCColorant(t *testing.T, profile []byte, name string) {
	t.Helper()
	offset, size, _ := findICCTag(profile, name)
	if string(profile[offset:offset+4]) != "XYZ " {
		t.Fatalf("tag %s type %q", name, profile[offset:offset+4])
	}
	if size != 20 {
		t.Fatalf("tag %s size %d, want 20", name, size)
	}
}

// findICCTag returns the data offset and size of one tag, checking its bounds.
func findICCTag(profile []byte, name string) (int, int, bool) {
	count := int(binary.BigEndian.Uint32(profile[iccHeaderSize:]))
	for i := range count {
		row := profile[iccHeaderSize+4+i*iccTagRowSize:]
		if string(row[0:4]) != name {
			continue
		}
		offset := int(binary.BigEndian.Uint32(row[4:]))
		size := int(binary.BigEndian.Uint32(row[8:]))
		if offset < iccHeaderSize || size == 0 || offset+size > len(profile) {
			return 0, 0, false
		}
		return offset, size, true
	}
	return 0, 0, false
}
