package pdf

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
)

// TestMarkedContentRead proves BMC, BDC, EMC, MP, and DP parse, that a marked
// page rasterizes as if the markers were absent, and that nesting is capped.
func TestMarkedContentRead(t *testing.T) {
	base := paintImage(t, "0 0 m 10 0 l S")
	cases := []struct{ name, src string }{
		{name: "bmc", src: "/Span BMC 0 0 m 10 0 l S EMC"},
		{name: "bdc dict", src: "/P << /MCID 0 >> BDC 0 0 m 10 0 l S EMC"},
		{name: "mp", src: "/Artifact MP 0 0 m 10 0 l S"},
		{name: "dp", src: "/Note << /MCID 1 >> DP 0 0 m 10 0 l S"},
		{name: "nested", src: "/Document <</MCID 1>> BDC /Span BMC EMC" + " EMC 0 0 m 10 0 l S"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := paintImage(t, testCase.src)
			if !bytes.Equal(got.Pixels, base.Pixels) {
				t.Fatal("marked content changed the pixels")
			}
		})
	}
	t.Run("name properties", checkMarkedNameProperty)
	t.Run("unmatched", checkMarkedUnmatched)
	t.Run("depth cap", checkMarkedDepth)
}

// checkMarkedNameProperty resolves a /Properties name operand and paints the
// same pixels as the marker-free content.
func checkMarkedNameProperty(t *testing.T) {
	t.Helper()
	file := doPage(t, "/P /MC0 BDC 0 0 m 10 0 l S EMC",
		"<< /Properties << /MC0 5 0 R >> >>", "<< /MCID 0 >>")
	pixmap := graphics.NewPixmap(pageSide, pageSide)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	got := shown(t, pixmap)
	base := paintImage(t, "0 0 m 10 0 l S")
	if !bytes.Equal(got.Pixels, base.Pixels) {
		t.Fatal("a named property changed the pixels")
	}
}

func checkMarkedUnmatched(t *testing.T) {
	t.Helper()
	cases := []string{"EMC", "/P BMC EMC" + " EMC"}
	for _, src := range cases {
		err := Paint(t.Context(), []byte(src), graphics.NewPixmap(2, 2), 1)
		wantJobErr(t, err, syntaxOp, errSyntax)
	}
}

func checkMarkedDepth(t *testing.T) {
	t.Helper()
	ok := strings.Repeat("/P BMC ", maxMarkedDepth) + strings.Repeat("EMC ", maxMarkedDepth)
	if err := Paint(t.Context(), []byte(ok), graphics.NewPixmap(2, 2), 1); err != nil {
		t.Fatal(err)
	}
	over := strings.Repeat("/P BMC ", maxMarkedDepth+1) + strings.Repeat("EMC ", maxMarkedDepth+1)
	err := Paint(t.Context(), []byte(over), graphics.NewPixmap(2, 2), 1)
	wantJobErr(t, err, "BMC", nameLimit)
}

// markedEvent is one recorded sink call.
type markedEvent struct {
	kind  string
	tag   string
	depth int
	mcid  int64
}

// markedLog records marked-content events in call order.
type markedLog struct {
	events []markedEvent
}

func (log *markedLog) BeginMarkedContent(tag string, properties Value, depth int) {
	log.events = append(log.events, markedEvent{kind: "begin", tag: tag, depth: depth, mcid: eventMCID(properties)})
}

func (log *markedLog) EndMarkedContent(depth int) {
	log.events = append(log.events, markedEvent{kind: "end", tag: "", depth: depth, mcid: eventMCID(NullVal())})
}

// eventMCID returns the /MCID of a properties dictionary, or -1.
func eventMCID(properties Value) int64 {
	if properties.Kind != KindDict {
		return -1
	}
	mcid, ok := properties.IntEntry("MCID")
	if !ok {
		return -1
	}
	return int64(mcid)
}

// TestMarkedContentEvents proves BeginMarkedContent and EndMarkedContent pair
// in stream order with the resolved properties and the nesting depth, and that
// MP and DP fire no event.
func TestMarkedContentEvents(t *testing.T) {
	file := doPage(t, markedEventsContent, "<< /Properties << /MC0 5 0 R >> >>", "<< /MCID 7 >>")
	res, err := file.PageResources(0)
	if err != nil {
		t.Fatal(err)
	}
	content, err := file.Content(0)
	if err != nil {
		t.Fatal(err)
	}
	log := &markedLog{}
	if err := PaintWith(t.Context(), content, nil, 1, PaintOptions{Resources: res, MarkedContent: log}); err != nil {
		t.Fatal(err)
	}
	want := []markedEvent{
		{kind: "begin", tag: "Document", depth: 1, mcid: 1},
		{kind: "begin", tag: "P", depth: 2, mcid: 2},
		{kind: "begin", tag: "Span", depth: 3, mcid: -1},
		{kind: "end", tag: "", depth: 3, mcid: -1},
		{kind: "end", tag: "", depth: 2, mcid: -1},
		{kind: "begin", tag: "Note", depth: 2, mcid: 7},
		{kind: "end", tag: "", depth: 2, mcid: -1},
		{kind: "end", tag: "", depth: 1, mcid: -1},
	}
	if len(log.events) != len(want) {
		t.Fatalf("events = %v, want %v", log.events, want)
	}
	for idx, event := range log.events {
		if event != want[idx] {
			t.Fatalf("event %d = %+v, want %+v", idx, event, want[idx])
		}
	}
	t.Run("typed nil", func(t *testing.T) {
		var typed *markedLog
		pixmap := graphics.NewPixmap(2, 2)
		err := PaintWith(t.Context(), []byte("/Span BMC EMC"), pixmap, 1, PaintOptions{MarkedContent: typed})
		if err != nil {
			t.Fatal(err)
		}
	})
}

const markedEventsContent = "/Document <</MCID 1>> BDC /P <</MCID 2>> BDC /Span BMC EMC EMC " +
	"/Note /MC0 BDC /Figure MP /Point << /MCID 3 >> DP EMC" + " EMC"

// TestRasterizeTaggedContent proves the tagged fixture that failed with
// undefined in BDC rasterizes and extracts with the markers absent.
func TestRasterizeTaggedContent(t *testing.T) {
	src, err := os.ReadFile("../../sampledata/pdfua2/tagged-ua2.pdf")
	if err != nil {
		t.Fatal(err)
	}
	file := mustOpen(t, src)
	pixmap := graphics.NewPixmap(100, 100)
	if err := file.PaintPage(t.Context(), 0, pixmap, 1); err != nil {
		t.Fatal(err)
	}
	img := shown(t, pixmap)
	if rowWhite(img, img.Height-1) {
		t.Fatal("the tagged fixture painted nothing")
	}
	text, err := file.ExtractText(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty", text)
	}
}
