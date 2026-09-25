package tag

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// tagDoc builds a classic-xref PDF with one empty content stream per page.
type tagDoc struct {
	buf     bytes.Buffer
	offsets []int
}

// newTagDoc returns an empty document writer.
func newTagDoc() *tagDoc {
	doc := &tagDoc{buf: bytes.Buffer{}, offsets: []int{0}}
	doc.buf.WriteString("%PDF-1.4\n")
	return doc
}

// object writes one body.
func (doc *tagDoc) object(body string) {
	num := len(doc.offsets)
	doc.offsets = append(doc.offsets, doc.buf.Len())
	fmt.Fprintf(&doc.buf, "%d 0 obj\n%s\nendobj\n", num, body)
}

// classic appends the xref table and the trailer.
func (doc *tagDoc) classic() []byte {
	xrefAt := doc.buf.Len()
	size := len(doc.offsets)
	fmt.Fprintf(&doc.buf, "xref\n0 %d\n", size)
	doc.buf.WriteString("0000000000 65535 f \n")
	for num := 1; num < size; num++ {
		fmt.Fprintf(&doc.buf, "%010d 00000 n \n", doc.offsets[num])
	}
	fmt.Fprintf(&doc.buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		size, xrefAt)
	return doc.buf.Bytes()
}

// openTagFixture builds a file with pages pages, opens it, and returns the
// file plus one recorder per page.
func openTagFixture(t *testing.T, pages int) (*pdf.File, []*Recorder) {
	t.Helper()
	doc := newTagDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kidRefs(pages), pages))
	for range pages {
		contentNum := len(doc.offsets) + 1
		doc.object(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] "+
			"/Contents %d 0 R /Resources << >> >>", contentNum))
		doc.object("<< /Length 0 >>\nstream\n\nendstream")
	}
	file, err := pdf.Open(t.Context(), doc.classic())
	if err != nil {
		t.Fatal(err)
	}
	recs := make([]*Recorder, pages)
	for index := range pages {
		recs[index] = recordStrokes(3)
	}
	return file, recs
}

// kidRefs lists the page object numbers of a fixture: page i is 3+2i.
func kidRefs(pages int) string {
	var buf bytes.Buffer
	for index := range pages {
		if index > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(&buf, "%d 0 R", 3+2*index)
	}
	return buf.String()
}

// openEmptyFixture builds a tagged fixture whose recorders start empty.
func openEmptyFixture(t *testing.T, pages int) (*pdf.File, []*Recorder) {
	t.Helper()
	file, recs := openTagFixture(t, pages)
	for index := range recs {
		recs[index] = NewRecorder()
	}
	return file, recs
}

// recordStrokes returns a recorder with count stroke events.
func recordStrokes(count int) *Recorder {
	rec := NewRecorder()
	for index := range count {
		rec.Stroke([]graphics.Point{
			{X: float64(index), Y: 0, Move: true},
			{X: float64(index) + 1, Y: 0, Move: false},
		}, 1, 0, 0, 0)
	}
	return rec
}

// recordRun records one synthetic text run at a device baseline. The box is
// half an em per rune wide and one em tall, which matches the reader's
// fallback geometry.
func recordRun(rec *Recorder, text string, posX, posY, size float64) {
	width := float64(len([]rune(text))) * size * 0.5
	codes := make([]uint32, 0, len([]rune(text)))
	units := make([]string, 0, len(codes))
	for _, unit := range text {
		codes = append(codes, uint32(unit))
		units = append(units, string(unit))
	}
	rec.TextRun(pdf.TextRun{
		FontName:   "F1",
		Size:       size,
		TextMatrix: graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: posX, F: posY},
		LineMatrix: graphics.Matrix{A: 1, B: 0, C: 0, D: 1, E: posX, F: posY},
		HScale:     1,
		Bytes:      []byte(text),
		Codes:      codes,
		Unicode:    units,
		CTM:        graphics.Identity(),
		Scale:      1,
		Box: pdf.Box{
			MinX: posX, MinY: posY - size*0.25,
			MaxX: posX + width, MaxY: posY + size*0.75,
		},
	})
}

// recordImage records one image event with its own /Alt entry.
func recordImage(rec *Recorder, alt string, box pdf.Box) {
	dict := pdf.DictVal(map[string]pdf.Value{})
	if alt != "" {
		dict = pdf.DictVal(map[string]pdf.Value{"Alt": pdf.StringVal(alt)})
	}
	rec.ImageName("Im0", dict)
	rec.DrawImage(image.NewRGBA(image.Rect(0, 0, 10, 10)), graphics.Identity(), 1)
	last := &rec.events[len(rec.events)-1]
	last.ImageBox = box
}

// planKids returns the top-level kids of one plan.
func planKids(t *testing.T, plan *Plan) []*Element {
	t.Helper()
	if plan == nil || plan.Root == nil {
		t.Fatal("plan or root is nil")
	}
	return plan.Root.Kids
}

// wantClaims fails when one element's claim list differs.
func wantClaims(t *testing.T, elem *Element, want []Claim) {
	t.Helper()
	if len(elem.Claims) != len(want) {
		t.Fatalf("%s claims = %+v, want %+v", elem.Type, elem.Claims, want)
	}
	for index := range want {
		if elem.Claims[index] != want[index] {
			t.Fatalf("%s claim %d = %+v, want %+v", elem.Type, index, elem.Claims[index], want[index])
		}
	}
}

// documentPlan returns a Document with one P child per claim list.
func documentPlan(claims [][]Claim) *Plan {
	kids := make([]*Element, 0, len(claims))
	for _, list := range claims {
		kids = append(kids, &Element{
			Type: "P", Alt: "", ActualText: "", Lang: "", Kids: nil, Claims: list,
		})
	}
	return &Plan{
		Root: &Element{
			Type: "Document", Alt: "", ActualText: "", Lang: "", Kids: kids, Claims: nil,
		},
		Roles: nil,
	}
}

// singleClaim returns the claim list for one half-open event range.
func singleClaim(page, first, last int) []Claim {
	return []Claim{{Page: page, First: first, Last: last}}
}
