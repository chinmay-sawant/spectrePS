package tag

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestTagReadingOrder proves a two-column page reads left column first, top
// to bottom, then right column.
func TestTagReadingOrder(t *testing.T) {
	file, recs := openEmptyFixture(t, 1)
	rec := recs[0]
	recordRun(rec, "left one", 20, 180, 12)
	recordRun(rec, "left two", 20, 168, 12)
	recordRun(rec, "left three", 20, 156, 12)
	recordRun(rec, "right one", 300, 174, 12)
	recordRun(rec, "right two", 300, 162, 12)
	recordRun(rec, "right three", 300, 150, 12)
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	if len(kids) != 2 {
		t.Fatalf("kids = %d, want 2", len(kids))
	}
	wantClaims(t, kids[0], []Claim{{Page: 0, First: 0, Last: 3}})
	wantClaims(t, kids[1], []Claim{{Page: 0, First: 3, Last: 6}})
	checkReadingOrderContent(t, file, recs, plan)
}

// checkReadingOrderContent proves the emitted MCIDs keep the plan order.
func checkReadingOrderContent(t *testing.T, file *pdf.File, recs []*Recorder, plan *Plan) {
	t.Helper()
	out := buildTagged(t, file, recs, plan, Options{Title: "Two columns", Lang: "en-US"})
	content := mustContent(t, reopen(t, out), 0)
	first := bytes.Index(content, []byte("/MCID 0"))
	second := bytes.Index(content, []byte("/MCID 1"))
	if first < 0 || second < 0 || first > second {
		t.Fatalf("MCID order in content: %d then %d", first, second)
	}
}

// TestTagGenerateParagraphs proves runs on one baseline with one gap between
// blocks become separate /P elements.
func TestTagGenerateParagraphs(t *testing.T) {
	_, recs := openEmptyFixture(t, 1)
	rec := recs[0]
	recordRun(rec, "first", 20, 180, 12)
	recordRun(rec, "second", 55, 180, 12)
	recordRun(rec, "next paragraph", 20, 120, 12)
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	if len(kids) != 2 {
		t.Fatalf("kids = %d, want 2", len(kids))
	}
	if kids[0].Type != "P" || kids[1].Type != "P" {
		t.Fatalf("types = %q %q", kids[0].Type, kids[1].Type)
	}
	wantClaims(t, kids[0], []Claim{{Page: 0, First: 0, Last: 2}})
	wantClaims(t, kids[1], []Claim{{Page: 0, First: 2, Last: 3}})
}

// TestTagHeadings proves a size a clear step above the body size becomes H1
// through H6, largest first.
func TestTagHeadings(t *testing.T) {
	_, recs := openEmptyFixture(t, 1)
	rec := recs[0]
	recordRun(rec, "Title", 20, 250, 24)
	recordRun(rec, "Section", 20, 200, 18)
	recordRun(rec, "Body one", 20, 150, 12)
	recordRun(rec, "Body two", 20, 138, 12)
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	want := []string{"H1", "H2", "P"}
	if len(kids) != len(want) {
		t.Fatalf("kids = %d, want %d", len(kids), len(want))
	}
	for index, name := range want {
		if kids[index].Type != name {
			t.Fatalf("kid %d = %q, want %q", index, kids[index].Type, name)
		}
	}
	wantClaims(t, kids[0], []Claim{{Page: 0, First: 0, Last: 1}})
	wantClaims(t, kids[2], []Claim{{Page: 0, First: 2, Last: 4}})
}

// TestTagLists proves bullet and number prefixes produce /L, /LI, /Lbl, and
// /LBody with the /ListNumbering attribute PDF/UA-2 requires.
func TestTagLists(t *testing.T) {
	t.Run("bullets", func(t *testing.T) {
		file, recs := openEmptyFixture(t, 1)
		rec := recs[0]
		recordRun(rec, "\u2022", 20, 180, 12)
		recordRun(rec, "First item", 35, 180, 12)
		recordRun(rec, "\u2022", 20, 168, 12)
		recordRun(rec, "Second item", 35, 168, 12)
		plan, err := DerivePlan(recs)
		if err != nil {
			t.Fatal(err)
		}
		checkListPlan(t, plan, "Disc")
		out := buildTagged(t, file, recs, plan, Options{Title: "List", Lang: "en-US"})
		if !bytes.Contains(out, []byte("/ListNumbering /Disc")) {
			t.Fatal("output lacks /ListNumbering /Disc")
		}
	})
	t.Run("numbers", func(t *testing.T) {
		_, recs := openEmptyFixture(t, 1)
		rec := recs[0]
		recordRun(rec, "1.", 20, 180, 12)
		recordRun(rec, "First item", 35, 180, 12)
		recordRun(rec, "2.", 20, 168, 12)
		recordRun(rec, "Second item", 35, 168, 12)
		plan, err := DerivePlan(recs)
		if err != nil {
			t.Fatal(err)
		}
		checkListPlan(t, plan, "Decimal")
	})
}

// checkListPlan proves the list shape and numbering of one plan.
func checkListPlan(t *testing.T, plan *Plan, numbering string) {
	t.Helper()
	kids := planKids(t, plan)
	if len(kids) != 1 || kids[0].Type != "L" {
		t.Fatalf("kids = %+v", kids)
	}
	if kids[0].ListNumbering != numbering {
		t.Fatalf("numbering = %q, want %q", kids[0].ListNumbering, numbering)
	}
	if len(kids[0].Kids) != 2 {
		t.Fatalf("items = %d, want 2", len(kids[0].Kids))
	}
	for index, item := range kids[0].Kids {
		if item.Type != "LI" || len(item.Kids) != 2 {
			t.Fatalf("item %d = %+v", index, item)
		}
		if item.Kids[0].Type != "Lbl" || item.Kids[1].Type != "LBody" {
			t.Fatalf("item %d kids = %q %q", index, item.Kids[0].Type, item.Kids[1].Type)
		}
	}
	wantClaims(t, kids[0].Kids[0].Kids[0], []Claim{{Page: 0, First: 0, Last: 1}})
	wantClaims(t, kids[0].Kids[0].Kids[1], []Claim{{Page: 0, First: 1, Last: 2}})
}

// TestTagTable proves aligned runs infer /Table, /TR, /TH, /TD, and /Scope.
func TestTagTable(t *testing.T) {
	file, recs := openEmptyFixture(t, 1)
	rec := recs[0]
	recordRun(rec, "Name", 20, 200, 12)
	recordRun(rec, "Value", 120, 200, 12)
	recordRun(rec, "Alpha", 20, 188, 12)
	recordRun(rec, "One", 120, 188, 12)
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	checkTablePlan(t, plan)
	checkTableScope(t, file, recs, plan)
}

// checkTablePlan proves the authored table shape.
func checkTablePlan(t *testing.T, plan *Plan) {
	t.Helper()
	kids := planKids(t, plan)
	if len(kids) != 1 || kids[0].Type != "Table" {
		t.Fatalf("kids = %+v", kids)
	}
	rows := kids[0].Kids
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for index, cell := range rows[0].Kids {
		if cell.Type != "TH" || cell.Scope != "Column" {
			t.Fatalf("header %d = %+v", index, cell)
		}
	}
	for index, cell := range rows[1].Kids {
		if cell.Type != "TD" || cell.Scope != "" {
			t.Fatalf("body %d = %+v", index, cell)
		}
	}
}

// checkTableScope proves the attribute object reaches the output.
func checkTableScope(t *testing.T, file *pdf.File, recs []*Recorder, plan *Plan) {
	t.Helper()
	out := buildTagged(t, file, recs, plan, Options{Title: "Table", Lang: "en-US"})
	if !bytes.Contains(out, []byte("/Scope /Column")) {
		t.Fatal("output lacks /Scope /Column")
	}
	if !bytes.Contains(out, []byte("/O /Table")) {
		t.Fatal("output lacks /O /Table")
	}
}

// TestTagFigureAlt proves the BDC property /Alt wins over the image
// dictionary /Alt, and a missing alt is /alt in Tag.
func TestTagFigureAlt(t *testing.T) {
	t.Run("property alt", func(t *testing.T) {
		_, recs := openEmptyFixture(t, 1)
		rec := recs[0]
		props := pdf.DictVal(map[string]pdf.Value{"Alt": pdf.StringVal("A chart of sales")})
		rec.BeginMarkedContent("Figure", props, 1)
		recordImage(rec, "Image alt", pdf.Box{MinX: 20, MinY: 100, MaxX: 120, MaxY: 160})
		rec.EndMarkedContent(1)
		checkFigurePlan(t, recs, "A chart of sales")
	})
	t.Run("image alt", func(t *testing.T) {
		_, recs := openEmptyFixture(t, 1)
		recordImage(recs[0], "Image alt", pdf.Box{MinX: 20, MinY: 100, MaxX: 120, MaxY: 160})
		checkFigurePlan(t, recs, "Image alt")
	})
	t.Run("missing alt", func(t *testing.T) {
		_, recs := openEmptyFixture(t, 1)
		recordImage(recs[0], "", pdf.Box{MinX: 20, MinY: 100, MaxX: 120, MaxY: 160})
		_, err := DerivePlan(recs)
		var job *pdf.Error
		if !errors.As(err, &job) || job.Op != opTag || job.Name != errAlt {
			t.Fatalf("error = %v, want /%s in %s", err, errAlt, opTag)
		}
	})
}

// checkFigurePlan proves one image authors a Figure with the wanted alt.
func checkFigurePlan(t *testing.T, recs []*Recorder, alt string) {
	t.Helper()
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	if len(kids) != 1 || kids[0].Type != "Figure" {
		t.Fatalf("kids = %+v", kids)
	}
	if kids[0].Alt != alt {
		t.Fatalf("alt = %q, want %q", kids[0].Alt, alt)
	}
}

// TestTagImageOnlyPage proves an image-only page tags as one Figure and
// preflights clean.
func TestTagImageOnlyPage(t *testing.T) {
	file, recs := openEmptyFixture(t, 1)
	recordImage(recs[0], "A full page chart", pdf.Box{MinX: 0, MinY: 0, MaxX: 200, MaxY: 200})
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	if len(kids) != 1 || kids[0].Type != "Figure" {
		t.Fatalf("kids = %+v", kids)
	}
	out := buildTagged(t, file, recs, plan, Options{
		Title: "Image only", Lang: "en-US", Claim: true,
	})
	reopened := reopen(t, out)
	tree := mustTree(t, reopened)
	if len(tree.Keys()) != 1 {
		t.Fatalf("keys = %+v", tree.Keys())
	}
	if got := mustUA2(t, reopened); got.Part != "2" {
		t.Fatalf("claim = %+v", got)
	}
}

// TestTagActualText proves a ligature code point carries /ActualText and
// TaggedTextOK accepts the element.
func TestTagActualText(t *testing.T) {
	file, recs := openEmptyFixture(t, 1)
	rec := recs[0]
	rec.TextRun(pdf.TextRun{
		FontName:   "F1",
		Size:       12,
		TextMatrix: matrixAt(20, 180),
		LineMatrix: matrixAt(20, 180),
		HScale:     1,
		Bytes:      []byte{0x01},
		Codes:      []uint32{0x01},
		Unicode:    []string{"\uFB01"},
		CTM:        identityMatrix(),
		Scale:      1,
		Box:        pdf.Box{MinX: 20, MinY: 177, MaxX: 32, MaxY: 189},
	})
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	kids := planKids(t, plan)
	if len(kids) != 1 || kids[0].ActualText != "fi" {
		t.Fatalf("kids = %+v", kids)
	}
	out := buildTagged(t, file, recs, plan, Options{Title: "Ligature", Lang: "en-US"})
	reopened := reopen(t, out)
	tree := mustTree(t, reopened)
	root := tree.Root()
	if root == nil || len(root.Kids) != 1 {
		t.Fatalf("root = %+v", root)
	}
	elem := root.Kids[0]
	if elem.ActualText != "fi" {
		t.Fatalf("ActualText = %q", elem.ActualText)
	}
	if !pdf.TaggedTextOK(elem, []pdf.Value{pdf.DictVal(map[string]pdf.Value{})}) {
		t.Fatal("TaggedTextOK = false with ActualText")
	}
	if pdf.TaggedTextOK(nil, []pdf.Value{pdf.DictVal(map[string]pdf.Value{})}) {
		t.Fatal("TaggedTextOK = true without a mapping or ActualText")
	}
}

// TestTagArtifacts proves a rule and repeated furniture wrap in /Artifact and
// stay out of the structure tree.
func TestTagArtifacts(t *testing.T) {
	file, recs := openEmptyFixture(t, 2)
	first := recs[0]
	recordRun(first, "Confidential", 20, 250, 12)
	recordRun(first, "Body one", 20, 180, 12)
	first.Stroke([]graphics.Point{
		{X: 20, Y: 150, Move: true},
		{X: 180, Y: 150, Move: false},
	}, 1, 0, 0, 0)
	second := recs[1]
	recordRun(second, "Confidential", 20, 250, 12)
	recordRun(second, "Body two", 20, 180, 12)
	plan, err := DerivePlan(recs)
	if err != nil {
		t.Fatal(err)
	}
	if len(planKids(t, plan)) != 2 {
		t.Fatalf("kids = %+v", planKids(t, plan))
	}
	if len(plan.Artifacts) != 3 {
		t.Fatalf("artifacts = %+v", plan.Artifacts)
	}
	out := buildTagged(t, file, recs, plan, Options{Title: "Artifacts", Lang: "en-US"})
	reopened := reopen(t, out)
	if got := len(mustTree(t, reopened).Keys()); got != 2 {
		t.Fatalf("keys = %d, want 2", got)
	}
	page0 := mustContent(t, reopened, 0)
	if got := bytes.Count(page0, []byte("/Artifact")); got != 2 {
		t.Fatalf("page 0 artifacts = %d, want 2", got)
	}
	if !strings.Contains(string(page0), "BMC\n") {
		t.Fatalf("page 0 has no artifact BMC: %q", page0)
	}
}

// matrixAt returns one translation matrix.
func matrixAt(x, y float64) graphics.Matrix {
	return graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: x, F: y}
}

// identityMatrix is the neutral matrix.
func identityMatrix() graphics.Matrix {
	return graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}
