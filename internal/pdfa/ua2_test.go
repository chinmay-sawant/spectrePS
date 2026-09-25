package pdfa

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
)

// ua2Fixture is the mutable shape of the tagged test file.
type ua2Fixture struct {
	marked          bool
	tree            bool
	documentType    string
	documentNS      bool
	lang            string
	displayDocTitle bool
	part            string
	rev             string
	title           string
	parentNums      string
	extra           []string
}

// defaultUA2Fixture is the shape every check accepts.
func defaultUA2Fixture() ua2Fixture {
	return ua2Fixture{
		marked:          true,
		tree:            true,
		documentType:    typeDocumentUA2,
		documentNS:      true,
		lang:            "en-US",
		displayDocTitle: true,
		part:            ua2PartValue,
		rev:             ua2RevValue,
		title:           "Annual report",
		parentNums:      "0 [7 0 R]",
		extra:           nil,
	}
}

// ua2File builds and opens the tagged test file.
func ua2File(t *testing.T, fix ua2Fixture) *pdf.File {
	t.Helper()
	packet := UA2XMP(UA2Metadata{Title: fix.title, Lang: fix.lang, Part: fix.part, Rev: fix.rev})
	objects := []string{
		ua2CatalogBody(fix),
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << >> /StructParents 0 >>",
		ua2Stream("/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC"),
		"<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 8 0 R /ParentTreeNextKey 1 >>",
		ua2DocumentBody(fix),
		"<< /Type /StructElem /S /Figure /Alt (A figure) /P 6 0 R /Pg 3 0 R /K 0 >>",
		"<< /Nums [" + fix.parentNums + "] >>",
		ua2Stream(string(packet)),
		"<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>",
	}
	objects = append(objects, fix.extra...)
	src := buildClassicPDF(t, objects)
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// ua2CatalogBody writes the catalog for one fixture shape.
func ua2CatalogBody(fix ua2Fixture) string {
	body := "<< /Type /Catalog /Pages 2 0 R"
	if fix.marked {
		body += " /MarkInfo << /Marked true >>"
	}
	if fix.tree {
		body += " /StructTreeRoot 5 0 R"
	}
	if fix.lang != "" {
		body += " /Lang (" + fix.lang + ")"
	}
	if fix.displayDocTitle {
		body += " /ViewerPreferences << /DisplayDocTitle true >>"
	}
	return body + " /Metadata 9 0 R >>"
}

// ua2DocumentBody writes the top structure element for one fixture shape.
func ua2DocumentBody(fix ua2Fixture) string {
	body := "<< /Type /StructElem /S /" + fix.documentType + " /P 5 0 R"
	if fix.documentNS {
		body += " /NS 10 0 R"
	}
	return body + " /K [7 0 R] >>"
}

// ua2Stream wraps one stream body.
func ua2Stream(content string) string {
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

func TestUA2Metadata(t *testing.T) {
	t.Run("read", checkUA2Read)
	t.Run("write", checkUA2Write)
	t.Run("claim gate", checkUA2ClaimGate)
}

func checkUA2Read(t *testing.T) {
	t.Helper()
	info := mustReadUA2(t, ua2File(t, defaultUA2Fixture()))
	if !info.Metadata || info.Part != ua2PartValue || info.Rev != ua2RevValue {
		t.Fatalf("claim = %+v", info)
	}
	if info.Title != "Annual report" || info.Lang != "en-US" {
		t.Fatalf("text = %+v", info)
	}
	if !info.Marked || info.Suspects || !info.DisplayDocTitle {
		t.Fatalf("flags = %+v", info)
	}
}

func checkUA2Write(t *testing.T) {
	t.Helper()
	file := ua2File(t, defaultUA2Fixture())
	info := mustReadUA2(t, file)
	if got := mustReadUA2(t, ua2Rewrite(t, file, UA2Write(info, false, false))); got != info {
		t.Fatalf("round trip = %+v, want %+v", got, info)
	}
	noClaim := UA2Info{
		Metadata:        true,
		Part:            "",
		Rev:             "",
		Title:           "Annual report",
		Lang:            "en-US",
		Marked:          true,
		Suspects:        false,
		DisplayDocTitle: true,
	}
	meta := UA2Write(noClaim, false, false)
	if meta.Part != "" {
		t.Fatalf("Part = %q without an opt-in", meta.Part)
	}
	if packet := string(UA2XMP(meta)); strings.Contains(packet, ua2PartTag) {
		t.Fatalf("packet carries a claim: %q", packet)
	}
	got := mustReadUA2(t, ua2Rewrite(t, file, meta))
	if got.Part != "" || got.Rev != "" {
		t.Fatalf("claim written without an opt-in: %+v", got)
	}
	if got.Title != noClaim.Title || got.Lang != noClaim.Lang {
		t.Fatalf("text lost: %+v", got)
	}
	if entries := UA2ExtraObjects(meta, 0); entries != nil {
		t.Fatalf("ExtraObjects(0) = %d entries", len(entries))
	}
}

func checkUA2ClaimGate(t *testing.T) {
	t.Helper()
	empty := UA2Info{
		Metadata:        true,
		Part:            "",
		Rev:             "",
		Title:           "Annual report",
		Lang:            "en-US",
		Marked:          true,
		Suspects:        false,
		DisplayDocTitle: true,
	}
	source := empty
	source.Part = ua2PartValue
	source.Rev = ua2RevValue
	cases := []struct {
		name   string
		info   UA2Info
		optIn  bool
		passed bool
		part   string
	}{
		{name: "source claim", info: source, optIn: false, passed: false, part: ua2PartValue},
		{name: "source claim, preflight failed", info: source, optIn: true, passed: false, part: ua2PartValue},
		{name: "opt in passed", info: empty, optIn: true, passed: true, part: ua2PartValue},
		{name: "opt in failed", info: empty, optIn: true, passed: false, part: ""},
		{name: "no opt in", info: empty, optIn: false, passed: false, part: ""},
		{name: "passed without opt in", info: empty, optIn: false, passed: true, part: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := UA2Write(testCase.info, testCase.optIn, testCase.passed)
			if got.Part != testCase.part {
				t.Fatalf("Part = %q, want %q", got.Part, testCase.part)
			}
		})
	}
}

// mustReadUA2 reads the metadata or fails the test.
func mustReadUA2(t *testing.T, file *pdf.File) UA2Info {
	t.Helper()
	info, err := ReadUA2(file)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// ua2Rewrite appends the metadata and overrides the catalog, the way a UA-2
// write does, and returns the reopened result.
func ua2Rewrite(t *testing.T, file *pdf.File, meta UA2Metadata) *pdf.File {
	t.Helper()
	catalog, err := ua2Catalog(file)
	if err != nil {
		t.Fatal(err)
	}
	first := file.ObjectCount() + 1
	out, err := pdfout.WriteCopy(t.Context(), file, pdfout.CopyOptions{
		Overrides:       nil,
		PackObjects:     false,
		AppendObjects:   UA2ExtraObjects(meta, first),
		CatalogOverride: UA2Catalog(catalog, first, meta),
		PDFA:            false,
	})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := pdf.Open(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	return reopened
}
