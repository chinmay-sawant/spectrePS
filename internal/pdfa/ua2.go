package pdfa

// This file reads and writes the PDF/UA-2 accessibility metadata: the catalog
// /Metadata stream with its XMP packet, pdfuaid, dc:title, /Lang, /MarkInfo,
// and /ViewerPreferences. A pdfuaid claim is written only when the source
// carried one or the caller opted in after a passing UA-2 preflight.

import (
	"fmt"
	"strings"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The XMP tag names and values PDF/UA-2 uses, and the catalog entries the
// metadata reader and writer touch.
const (
	ua2PartValue = "2"
	ua2RevValue  = "2024"
	ua2PartTag   = "pdfuaid:part"
	ua2RevTag    = "pdfuaid:rev"

	opPDFUA = "PDFUA"

	keyMetadata          = "Metadata"
	keyUA2Lang           = "Lang"
	keyUA2MarkInfo       = "MarkInfo"
	keyUA2Marked         = "Marked"
	keyUA2Suspects       = "Suspects"
	keyUA2ViewerPrefs    = "ViewerPreferences"
	keyUA2DisplayTitle   = "DisplayDocTitle"
	keyUA2DecodeParms    = "DecodeParms"
	nameUA2XML           = "XML"
	nameUA2Flate         = "FlateDecode"
	ua2XMPDescriptionEnd = "  </rdf:Description>\n"
	typeDocumentUA2      = "Document"
	ua2CatalogExtra      = 4
)

// UA2Info is the accessibility metadata read from one document.
type UA2Info struct {
	// Metadata reports a catalog /Metadata stream.
	Metadata bool
	// Part is the pdfuaid:part value, and Rev is pdfuaid:rev. Both are empty
	// when the XMP carries no claim.
	Part string
	Rev  string
	// Title is the dc:title text, empty when the XMP carries none.
	Title string
	// Lang is the catalog /Lang.
	Lang string
	// Marked and Suspects are the /MarkInfo values.
	Marked   bool
	Suspects bool
	// DisplayDocTitle is /ViewerPreferences /DisplayDocTitle.
	DisplayDocTitle bool
}

// UA2Metadata is the accessibility metadata a UA-2 write sets. An empty Part
// writes no pdfuaid claim, and an empty Rev writes no pdfuaid:rev element.
type UA2Metadata struct {
	Title string
	Lang  string
	Part  string
	Rev   string
}

// ReadUA2 returns the accessibility metadata of one file. A nil file returns
// the zero value. The XMP packet is read from the catalog /Metadata stream;
// an unfiltered stream and a Flate stream are read, and any other filter
// yields no packet text.
func ReadUA2(file *pdf.File) (UA2Info, error) {
	if file == nil {
		return zeroUA2Info(), nil
	}
	catalog, err := ua2Catalog(file)
	if err != nil {
		return zeroUA2Info(), err
	}
	info := zeroUA2Info()
	info.Lang = ua2String(catalog, keyUA2Lang)
	info.Marked = ua2SubBool(file, catalog, keyUA2MarkInfo, keyUA2Marked)
	info.Suspects = ua2SubBool(file, catalog, keyUA2MarkInfo, keyUA2Suspects)
	info.DisplayDocTitle = ua2SubBool(file, catalog, keyUA2ViewerPrefs, keyUA2DisplayTitle)
	entry, ok := catalog.ValueEntry(keyMetadata)
	if !ok || entry.Kind == pdf.KindNull {
		return info, nil
	}
	info.Metadata = true
	packet, err := ua2Packet(file, entry)
	if err != nil {
		return info, err
	}
	info.Part = ua2TagValue(packet, ua2PartTag)
	info.Rev = ua2TagValue(packet, ua2RevTag)
	info.Title = ua2TitleValue(packet)
	return info, nil
}

// UA2Write returns the metadata one rewrite writes. The claim is the source
// part and rev when the source carried pdfuaid. A source without a claim gets
// part 2 and rev 2024 only when the caller opted in and the UA-2 preflight
// passed. Otherwise no claim is written.
func UA2Write(info UA2Info, optIn, preflightPassed bool) UA2Metadata {
	meta := UA2Metadata{Title: info.Title, Lang: info.Lang, Part: "", Rev: ""}
	switch {
	case info.Part != "":
		meta.Part = info.Part
		meta.Rev = info.Rev
	case optIn && preflightPassed:
		meta.Part = ua2PartValue
		meta.Rev = ua2RevValue
	}
	return meta
}

// UA2XMP returns the UTF-8 XMP packet for a UA-2 write: pdfuaid when meta
// carries a claim, and dc:title. The packet carries no dates.
func UA2XMP(meta UA2Metadata) []byte {
	var buf strings.Builder
	buf.WriteString(ua2XMPHead)
	if meta.Part != "" {
		fmt.Fprintf(&buf, "   <pdfuaid:part>%s</pdfuaid:part>\n", ua2Escape(meta.Part))
		if meta.Rev != "" {
			fmt.Fprintf(&buf, "   <pdfuaid:rev>%s</pdfuaid:rev>\n", ua2Escape(meta.Rev))
		}
	}
	buf.WriteString(ua2XMPDescriptionEnd)
	buf.WriteString(ua2XMPTitleHead)
	fmt.Fprintf(&buf, "     <rdf:li xml:lang=\"x-default\">%s</rdf:li>\n", ua2Escape(meta.Title))
	buf.WriteString(ua2XMPTitleTail)
	buf.WriteString(ua2XMPTail)
	return []byte(buf.String())
}

// UA2ExtraObjects returns the complete bodies a UA-2 write appends, in order:
// the XMP metadata stream. firstNum is the object number of the metadata
// stream. A firstNum below 1 returns nil.
func UA2ExtraObjects(meta UA2Metadata, firstNum int) [][]byte {
	if firstNum < 1 {
		return nil
	}
	metadata := pdf.SerializeValue(pdf.StreamVal(map[string]pdf.Value{
		"Type":    pdf.NameVal(keyMetadata),
		"Subtype": pdf.NameVal(nameUA2XML),
	}, UA2XMP(meta)))
	return [][]byte{metadata}
}

// UA2Catalog returns a replacement catalog body for a UA-2 write: /Metadata
// points at the appended stream, /Lang is written when meta carries one,
// /MarkInfo /Marked is true, and /ViewerPreferences /DisplayDocTitle is true.
// Every other entry is kept. A source entry that is an indirect reference is
// replaced by a direct dictionary.
func UA2Catalog(catalog pdf.Value, metadataNum int, meta UA2Metadata) []byte {
	entries := make(map[string]pdf.Value, len(catalog.Dict)+ua2CatalogExtra)
	for key, entry := range catalog.Dict {
		entries[key] = entry
	}
	if metadataNum >= 1 {
		entries[keyMetadata] = pdf.RefVal(metadataNum, 0)
	}
	if meta.Lang != "" {
		entries[keyUA2Lang] = pdf.StringVal(meta.Lang)
	}
	entries[keyUA2MarkInfo] = ua2MarkInfoDict(entries[keyUA2MarkInfo])
	entries[keyUA2ViewerPrefs] = ua2ViewerPrefsDict(entries[keyUA2ViewerPrefs])
	return pdf.SerializeValue(pdf.DictVal(entries))
}

// zeroUA2Info returns the empty metadata.
func zeroUA2Info() UA2Info {
	return UA2Info{
		Metadata:        false,
		Part:            "",
		Rev:             "",
		Title:           "",
		Lang:            "",
		Marked:          false,
		Suspects:        false,
		DisplayDocTitle: false,
	}
}

// ua2Catalog returns the resolved catalog dictionary.
func ua2Catalog(file *pdf.File) (pdf.Value, error) {
	root := file.RootNum()
	if root <= 0 {
		return pdf.NullVal(), pdf.NewError(opPDFUA, errSyntax)
	}
	catalog, ok, err := file.ObjectValue(root)
	if err != nil {
		return pdf.NullVal(), err
	}
	if !ok || catalog.Kind != pdf.KindDict {
		return pdf.NullVal(), pdf.NewError(opPDFUA, errSyntax)
	}
	return catalog, nil
}

// ua2SubBool reads one boolean sub-entry of a catalog entry.
func ua2SubBool(file *pdf.File, catalog pdf.Value, key, sub string) bool {
	entry, ok := catalog.ValueEntry(key)
	if !ok {
		return false
	}
	val, err := ua2Resolve(file, entry)
	if err != nil || val.Kind != pdf.KindDict {
		return false
	}
	flag, ok := val.ValueEntry(sub)
	return ok && flag.Kind == pdf.KindBool && flag.Bool
}

// ua2String reads one text string catalog entry.
func ua2String(catalog pdf.Value, key string) string {
	entry, ok := catalog.ValueEntry(key)
	if !ok || entry.Kind != pdf.KindString {
		return ""
	}
	return entry.String
}

// ua2Packet returns the XMP packet text of the catalog /Metadata value.
func ua2Packet(file *pdf.File, entry pdf.Value) (string, error) {
	val, err := ua2Resolve(file, entry)
	if err != nil {
		return "", err
	}
	if val.Kind != pdf.KindStream {
		return "", nil
	}
	raw, err := ua2StreamBytes(val)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ua2StreamBytes decodes an unfiltered or single Flate metadata stream. Any
// other filter returns nil, so the caller sees no packet text.
func ua2StreamBytes(val pdf.Value) ([]byte, error) {
	entry, ok := val.ValueEntry(keyFilter)
	if !ok || entry.Kind == pdf.KindNull {
		return val.Stream, nil
	}
	name, ok := ua2FilterName(entry)
	if !ok || name != nameUA2Flate {
		return nil, nil
	}
	params, _ := val.ValueEntry(keyUA2DecodeParms)
	return pdf.Decode(nameUA2Flate, params, val.Stream)
}

// ua2FilterName reads a filter name from a name or a one-name array.
func ua2FilterName(entry pdf.Value) (string, bool) {
	if entry.Kind == pdf.KindName {
		return entry.Name, true
	}
	if entry.Kind == pdf.KindArray && len(entry.Array) == 1 && entry.Array[0].Kind == pdf.KindName {
		return entry.Array[0].Name, true
	}
	return "", false
}

// ua2TagValue returns the text between one open and close tag.
func ua2TagValue(packet, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(packet, open)
	if start < 0 {
		return ""
	}
	rest := packet[start+len(open):]
	end := strings.Index(rest, closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(ua2Unescape(rest[:end]))
}

// ua2TitleValue returns the first rdf:li text inside the dc:title element.
func ua2TitleValue(packet string) string {
	start := strings.Index(packet, "<dc:title>")
	if start < 0 {
		return ""
	}
	rest := packet[start:]
	item := strings.Index(rest, "<rdf:li")
	if item < 0 {
		return ""
	}
	rest = rest[item:]
	open := strings.IndexByte(rest, '>')
	if open < 0 {
		return ""
	}
	rest = rest[open+1:]
	end := strings.Index(rest, "</rdf:li>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(ua2Unescape(rest[:end]))
}

// ua2MarkInfoDict copies one /MarkInfo dictionary and forces /Marked true.
func ua2MarkInfoDict(entry pdf.Value) pdf.Value {
	dict := ua2CopyDict(entry)
	dict[keyUA2Marked] = pdf.BoolVal(true)
	return pdf.DictVal(dict)
}

// ua2ViewerPrefsDict copies one /ViewerPreferences dictionary and forces
// /DisplayDocTitle true.
func ua2ViewerPrefsDict(entry pdf.Value) pdf.Value {
	dict := ua2CopyDict(entry)
	dict[keyUA2DisplayTitle] = pdf.BoolVal(true)
	return pdf.DictVal(dict)
}

// ua2CopyDict copies one direct dictionary, and returns an empty map for
// anything else.
func ua2CopyDict(entry pdf.Value) map[string]pdf.Value {
	dict := map[string]pdf.Value{}
	if entry.Kind != pdf.KindDict {
		return dict
	}
	for key, val := range entry.Dict {
		dict[key] = val
	}
	return dict
}

// ua2Resolve dereferences one indirect value.
func ua2Resolve(file *pdf.File, val pdf.Value) (pdf.Value, error) {
	if val.Kind != pdf.KindRef {
		return val, nil
	}
	resolved, ok, err := file.ObjectValue(val.RefNum)
	if err != nil {
		return pdf.NullVal(), err
	}
	if !ok {
		return pdf.NullVal(), nil
	}
	return resolved, nil
}

// ua2Escape escapes the XML text entities the packet can carry.
func ua2Escape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// ua2Unescape reverses ua2Escape and the quote entities.
func ua2Unescape(text string) string {
	return strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
		"&amp;", "&",
	).Replace(text)
}

// The static XMP packet parts. They carry no dates.
const (
	ua2XMPHead = "<?xpacket begin=\"\uFEFF\" id=\"W5M0MpCehiHzreSzNTczkc9d\"?>\n" +
		"<x:xmpmeta xmlns:x=\"adobe:ns:meta/\">\n" +
		" <rdf:RDF xmlns:rdf=\"http://www.w3.org/1999/02/22-rdf-syntax-ns#\">\n" +
		"  <rdf:Description rdf:about=\"\" xmlns:pdfuaid=\"http://www.aiim.org/pdfua/ns/id/\">\n"

	ua2XMPTitleHead = "  <rdf:Description rdf:about=\"\" xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n" +
		"   <dc:title>\n" +
		"    <rdf:Alt>\n"

	ua2XMPTitleTail = "    </rdf:Alt>\n" +
		"   </dc:title>\n" +
		"  </rdf:Description>\n"

	ua2XMPTail = "  <rdf:Description rdf:about=\"\" xmlns:xmp=\"http://ns.adobe.com/xap/1.0/\">\n" +
		"   <xmp:CreatorTool>spectreps</xmp:CreatorTool>\n" +
		"  </rdf:Description>\n" +
		" </rdf:RDF>\n" +
		"</x:xmpmeta>\n" +
		"<?xpacket end=\"w\"?>\n"
)
