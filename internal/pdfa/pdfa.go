// Package pdfa builds the PDF/A-4 metadata a rewrite appends and preflights
// the input for violations the claim cannot carry.
// The result is a profile preflight, not a certificate.
package pdfa

import (
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// Mode is the archival profile a rewrite claims.
type Mode int

const (
	// ModeNone leaves the claim off.
	ModeNone Mode = iota
	// Mode4 is PDF/A-4 base. An input with embedded files is refused.
	Mode4
	// Mode4F is PDF/A-4f. An input with no embedded files is refused.
	Mode4F
)

// ExtraObjects returns the complete bodies a claim appends, in order: the XMP
// metadata stream, the ICC profile stream, and the output intent dictionary.
// firstNum is the object number of the metadata stream, so the ICC stream is
// firstNum+1 and the intent is firstNum+2.
// A mode other than Mode4 or Mode4F returns nil.
func ExtraObjects(mode Mode, firstNum int) [][]byte {
	packet := XMP(mode)
	if packet == nil || firstNum < 1 {
		return nil
	}
	metadata := pdf.SerializeValue(pdf.StreamVal(map[string]pdf.Value{
		"Type":    pdf.NameVal("Metadata"),
		"Subtype": pdf.NameVal("XML"),
	}, packet))
	profile := pdf.SerializeValue(pdf.StreamVal(map[string]pdf.Value{
		"N": pdf.IntVal(iccComponentNum),
	}, ICCProfile()))
	intent := pdf.SerializeValue(pdf.DictVal(map[string]pdf.Value{
		"Type":                      pdf.NameVal("OutputIntent"),
		"S":                         pdf.NameVal("GTS_PDFA1"),
		"OutputConditionIdentifier": pdf.StringVal("sRGB IEC61966-2.1"),
		"Info":                      pdf.StringVal("sRGB IEC61966-2.1"),
		"DestOutputProfile":         pdf.RefVal(firstNum+1, 0),
	}))
	return [][]byte{metadata, profile, intent}
}

// Catalog returns a replacement catalog body with /Metadata and
// /OutputIntents set to the appended objects. Every other entry is kept.
func Catalog(catalog pdf.Value, metadataNum, intentNum int) []byte {
	entries := make(map[string]pdf.Value, len(catalog.Dict)+iccExtraEntries)
	for key, entry := range catalog.Dict {
		entries[key] = entry
	}
	entries["Metadata"] = pdf.RefVal(metadataNum, 0)
	entries["OutputIntents"] = pdf.ArrayVal([]pdf.Value{pdf.RefVal(intentNum, 0)})
	return pdf.SerializeValue(pdf.DictVal(entries))
}
