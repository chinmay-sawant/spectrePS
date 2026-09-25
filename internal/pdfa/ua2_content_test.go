package pdfa

import (
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestUA2ContentCoverage proves the content-side rule: paired, covered
// content passes, and an uncovered, unpaired, or claim-only MCID is
// ua2-content.
func TestUA2ContentCoverage(t *testing.T) {
	t.Run("covered", checkUA2Covered)
	t.Run("artifact", checkUA2Artifact)
	t.Run("uncovered content", checkUA2UncoveredContent)
	t.Run("unpaired content", checkUA2UnpairedContent)
	t.Run("orphan claim", checkUA2OrphanClaim)
}

// checkUA2Covered is the positive fixture: one BDC with MCID 0 and an
// agreeing parent tree entry.
func checkUA2Covered(t *testing.T) {
	t.Helper()
	file := ua2ContentFile(t, "/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC", "0 [7 0 R]", "/K 0")
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
}

// checkUA2Artifact proves a marked-content item with no MCID, the artifact
// case, is not uncovered.
func checkUA2Artifact(t *testing.T) {
	t.Helper()
	file := ua2ContentFile(t, "/Artifact BMC 0 0 m 10 0 l S EMC", "", "/K []")
	wantNoUA2Rule(t, PreflightUA2(t.Context(), file))
}

// checkUA2UncoveredContent proves an MCID no element claims is ua2-content.
func checkUA2UncoveredContent(t *testing.T) {
	t.Helper()
	file := ua2ContentFile(t, "/P <</MCID 0>> BDC 0 0 m 10 0 l S EMC", "", "/K []")
	wantUA2Rule(t, PreflightUA2(t.Context(), file), ruleUA2Content)
}

// checkUA2UnpairedContent proves an open BDC with no EMC is ua2-content.
func checkUA2UnpairedContent(t *testing.T) {
	t.Helper()
	file := ua2ContentFile(t, "/P <</MCID 0>> BDC 0 0 m 10 0 l S", "0 [7 0 R]", "/K 0")
	wantUA2Rule(t, PreflightUA2(t.Context(), file), ruleUA2Content)
}

// checkUA2OrphanClaim proves a parent tree claim with no content item is
// ua2-content.
func checkUA2OrphanClaim(t *testing.T) {
	t.Helper()
	file := ua2ContentFile(t, "0 0 m 10 0 l S", "0 [7 0 R]", "/K 0")
	wantUA2Rule(t, PreflightUA2(t.Context(), file), ruleUA2Content)
}

// ua2ContentFile builds the conforming fixture with one custom content
// stream, parent tree /Nums body, and element /K entry.
func ua2ContentFile(t *testing.T, content, nums, kid string) *pdf.File {
	t.Helper()
	packet := UA2XMP(UA2Metadata{Title: "Annual report", Lang: "en-US", Part: "2", Rev: "2024"})
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /MarkInfo << /Marked true >> " +
			"/StructTreeRoot 5 0 R /Lang (en-US) " +
			"/ViewerPreferences << /DisplayDocTitle true >> /Metadata 9 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R " +
			"/Resources << >> /StructParents 0 >>",
		ua2Stream(content),
		"<< /Type /StructTreeRoot /K [6 0 R] /ParentTree 8 0 R /ParentTreeNextKey 1 >>",
		"<< /Type /StructElem /S /Document /P 5 0 R /NS 10 0 R /K [7 0 R] >>",
		"<< /Type /StructElem /S /Figure /Alt (A figure) /P 6 0 R /Pg 3 0 R " + kid + " >>",
		"<< /Nums [" + nums + "] >>",
		ua2Stream(string(packet)),
		"<< /Type /Namespace /NS (http://iso.org/pdf2/ssn) >>",
	}
	src := buildClassicPDF(t, objects)
	file, err := pdf.Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}
