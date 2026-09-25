package pdfa

// The packet is static so two rewrites of the same input return equal bytes.
// It carries no dates. The F conformance letter appears only in PDF/A-4f.
const (
	xmpHead = "<?xpacket begin=\"\uFEFF\" id=\"W5M0MpCehiHzreSzNTczkc9d\"?>\n" +
		"<x:xmpmeta xmlns:x=\"adobe:ns:meta/\">\n" +
		" <rdf:RDF xmlns:rdf=\"http://www.w3.org/1999/02/22-rdf-syntax-ns#\">\n" +
		"  <rdf:Description rdf:about=\"\" xmlns:pdfaid=\"http://www.aiim.org/pdfa/ns/id/\">\n" +
		"   <pdfaid:part>4</pdfaid:part>\n" +
		"   <pdfaid:rev>2020</pdfaid:rev>\n"

	xmpConformanceF = "   <pdfaid:conformance>F</pdfaid:conformance>\n"

	xmpTail = "  </rdf:Description>\n" +
		"  <rdf:Description rdf:about=\"\" xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n" +
		"   <dc:title>\n" +
		"    <rdf:Alt>\n" +
		"     <rdf:li xml:lang=\"x-default\">Spectre PS rewrite</rdf:li>\n" +
		"    </rdf:Alt>\n" +
		"   </dc:title>\n" +
		"  </rdf:Description>\n" +
		"  <rdf:Description rdf:about=\"\" xmlns:xmp=\"http://ns.adobe.com/xap/1.0/\">\n" +
		"   <xmp:CreatorTool>spectreps</xmp:CreatorTool>\n" +
		"  </rdf:Description>\n" +
		" </rdf:RDF>\n" +
		"</x:xmpmeta>\n" +
		"<?xpacket end=\"w\"?>\n"
)

// XMP returns the static UTF-8 XMP packet for mode: pdfaid:part 4, pdfaid:rev
// 2020, and the F conformance letter for Mode4F. The packet carries no dates.
// A mode other than Mode4 or Mode4F returns nil.
func XMP(mode Mode) []byte {
	switch mode {
	case Mode4:
		return []byte(xmpHead + xmpTail)
	case Mode4F:
		return []byte(xmpHead + xmpConformanceF + xmpTail)
	default:
		return nil
	}
}
