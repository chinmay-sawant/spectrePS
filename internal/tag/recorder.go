// Package tag records PDF content events, authors a structure tree, and
// writes a tagged PDF 2.0 file. It generates tags and preflights the result
// with internal/pdfa; it never certifies.
package tag

import (
	"bytes"
	"image"
	"slices"
	"strconv"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The tag job error names. The public layer maps them to JobError.
const (
	opTag       = "Tag"
	errPlan     = "plan"
	errMCID     = "mcid"
	errRoleMap  = "rolemap"
	errImage    = "image"
	percentFull = 100
)

// MarkedContentSink receives marked-content boundaries.
type MarkedContentSink = pdf.MarkedContentSink

// TextRun is one shown string with the text state at show time. It is an
// alias, so Recorder.TextRun satisfies pdf.TextRunSink exactly. One run is
// one shown string: a TJ array fires one run per string, and the captured
// TextMatrix reproduces the placement.
type TextRun = pdf.TextRun

// TextRunSink receives one shown string. A marker that implements it accepts
// text without a glyph outline source.
type TextRunSink = pdf.TextRunSink

// ImageNameMarker receives one XObject name and dictionary before decode.
type ImageNameMarker = pdf.ImageNameMarker

// EventKind names one recorded content event.
type EventKind int

const (
	// EventStroke is one stroked path.
	EventStroke EventKind = iota
	// EventFill is one filled path.
	EventFill
	// EventImage is one painted image XObject.
	EventImage
	// EventText is one shown string.
	EventText
	// EventBeginMarked is one BMC or BDC.
	EventBeginMarked
	// EventEndMarked is one EMC.
	EventEndMarked
)

// Event is one recorded content event in stream order.
type Event struct {
	// Kind selects the fields that carry meaning.
	Kind EventKind
	// Points is the device-space path of a stroke or fill.
	Points []graphics.Point
	// Width is the stroke width in device pixels.
	Width float64
	// Red, Green, and Blue are the paint color.
	Red   float64
	Green float64
	Blue  float64
	// EvenOdd selects the even-odd fill rule.
	EvenOdd bool
	// ImageName is the /XObject name, and ImageDict is its resolved
	// dictionary, for an image event.
	ImageName string
	ImageDict pdf.Value
	// Text is the shown run of a text event.
	Text TextRun
	// Tag and Properties are the marked-content boundary of a marked event.
	Tag        string
	Properties pdf.Value
	// Depth is the marked-content nesting depth of a marked event.
	Depth int
}

// Recorder implements graphics.Marker, MarkedContentSink, TextRunSink, and
// ImageNameMarker, and records events in stream order. It writes no operators
// itself: Content and the tree builder serialize the events. The recorder
// carries no page resources, so it cannot paint; a text run and an image name
// are re-emitted as Tj and Do.
type Recorder struct {
	events       []Event
	err          error
	imagePending bool
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{events: nil, err: nil, imagePending: false}
}

// Stroke records one stroked path.
func (rec *Recorder) Stroke(pts []graphics.Point, width, red, green, blue float64) {
	evt := newEvent(EventStroke)
	evt.Points = slices.Clone(pts)
	evt.Width = width
	evt.Red = red
	evt.Green = green
	evt.Blue = blue
	rec.events = append(rec.events, evt)
}

// Fill records one filled path.
func (rec *Recorder) Fill(pts []graphics.Point, red, green, blue float64, evenOdd bool) {
	evt := newEvent(EventFill)
	evt.Points = slices.Clone(pts)
	evt.Red = red
	evt.Green = green
	evt.Blue = blue
	evt.EvenOdd = evenOdd
	rec.events = append(rec.events, evt)
}

// DrawImage consumes one pending image name. A draw with no name came from a
// runner without the ImageName seam, so the recorder refuses it instead of
// dropping the image.
func (rec *Recorder) DrawImage(_ image.Image, _ graphics.Matrix, _ float64) {
	if rec.imagePending {
		rec.imagePending = false
		return
	}
	rec.setErr(pdf.NewError(opTag, errImage))
}

// ImageName records one XObject name and dictionary before decode.
func (rec *Recorder) ImageName(name string, dict pdf.Value) {
	evt := newEvent(EventImage)
	evt.ImageName = name
	evt.ImageDict = dict
	rec.events = append(rec.events, evt)
	rec.imagePending = true
}

// BeginMarkedContent records one BMC or BDC.
func (rec *Recorder) BeginMarkedContent(tag string, properties pdf.Value, depth int) {
	evt := newEvent(EventBeginMarked)
	evt.Tag = tag
	evt.Properties = properties
	evt.Depth = depth
	rec.events = append(rec.events, evt)
}

// EndMarkedContent records one EMC.
func (rec *Recorder) EndMarkedContent(depth int) {
	evt := newEvent(EventEndMarked)
	evt.Depth = depth
	rec.events = append(rec.events, evt)
}

// TextRun records one shown string.
func (rec *Recorder) TextRun(run TextRun) {
	evt := newEvent(EventText)
	evt.Text = run
	evt.Text.Bytes = slices.Clone(run.Bytes)
	rec.events = append(rec.events, evt)
}

// Events returns the recorded events in stream order. The result is a copy of
// the slice; the nested path and byte slices are the recorder's.
func (rec *Recorder) Events() []Event {
	return slices.Clone(rec.events)
}

// Err returns the first refusal, or nil. DrawImage with no ImageName is the
// only refusal today.
func (rec *Recorder) Err() error {
	return rec.err
}

// Content returns the re-emitted operators with no generated MCIDs. Source
// marked-content operators are kept, so the bytes are a faithful event dump.
// An empty recording is a non-nil empty slice.
func (rec *Recorder) Content() []byte {
	var buf bytes.Buffer
	emitEvents(&buf, rec.events, nil, false)
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out
}

// setErr keeps the first refusal.
func (rec *Recorder) setErr(err error) {
	if rec.err == nil {
		rec.err = err
	}
}

// newEvent returns an empty event of one kind with every field set.
func newEvent(kind EventKind) Event {
	return Event{
		Kind:       kind,
		Points:     nil,
		Width:      0,
		Red:        0,
		Green:      0,
		Blue:       0,
		EvenOdd:    false,
		ImageName:  "",
		ImageDict:  pdf.NullVal(),
		Text:       zeroTextRun(),
		Tag:        "",
		Properties: pdf.NullVal(),
		Depth:      0,
	}
}

// zeroTextRun returns a run with the neutral text state: 100 percent scale
// and identity matrices.
func zeroTextRun() TextRun {
	return TextRun{
		FontName:    "",
		Size:        0,
		TextMatrix:  graphics.Identity(),
		LineMatrix:  graphics.Identity(),
		Rise:        0,
		CharSpacing: 0,
		WordSpacing: 0,
		HScale:      1,
		Bytes:       nil,
	}
}

// span is one generated marked-content sequence: a half-open range of events
// on one page, with its MCID and the tag name BDC writes.
type span struct {
	first int
	last  int
	mcid  int
	tag   string
	elem  *builtElem
}

// emitEvents writes the events. Every span wraps its range in
// /tag <</MCID n>> BDC ... EMC. skipMarked drops source marked-content events,
// so a generated tree owns every marked-content sequence it writes.
func emitEvents(buf *bytes.Buffer, events []Event, spans []span, skipMarked bool) {
	starts := make(map[int]span, len(spans))
	ends := make(map[int]span, len(spans))
	for _, mark := range spans {
		starts[mark.first] = mark
		ends[mark.last] = mark
	}
	for index, evt := range events {
		if _, ok := ends[index]; ok {
			buf.WriteString("EMC\n")
		}
		if mark, ok := starts[index]; ok {
			writeBeginGenerated(buf, mark)
		}
		if skipMarked && evt.marked() {
			continue
		}
		emitEvent(buf, evt)
	}
	if _, ok := ends[len(events)]; ok {
		buf.WriteString("EMC\n")
	}
}

// marked reports whether one event is a source marked-content boundary.
func (evt Event) marked() bool {
	return evt.Kind == EventBeginMarked || evt.Kind == EventEndMarked
}

// emitEvent writes one event's operators.
func emitEvent(buf *bytes.Buffer, evt Event) {
	switch evt.Kind {
	case EventStroke:
		writeStroke(buf, evt)
	case EventFill:
		writeFill(buf, evt)
	case EventImage:
		writeImage(buf, evt.ImageName)
	case EventText:
		writeTextRun(buf, evt.Text)
	case EventBeginMarked:
		writeBeginMarked(buf, evt)
	case EventEndMarked:
		buf.WriteString("EMC\n")
	}
}

// writeBeginGenerated writes one generated BDC with its MCID.
func writeBeginGenerated(buf *bytes.Buffer, mark span) {
	props := pdf.DictVal(map[string]pdf.Value{"MCID": pdf.IntVal(int64(mark.mcid))})
	writeOp(buf, nameText(mark.tag), string(pdf.SerializeValue(props)), "BDC")
}

// writeBeginMarked re-emits one source BMC or BDC.
func writeBeginMarked(buf *bytes.Buffer, evt Event) {
	if evt.Properties.Kind == pdf.KindNull {
		writeOp(buf, nameText(evt.Tag), "BMC")
		return
	}
	writeOp(buf, nameText(evt.Tag), string(pdf.SerializeValue(evt.Properties)), "BDC")
}

// writeStroke writes one stroked path the way the level 0 writer does.
func writeStroke(buf *bytes.Buffer, evt Event) {
	if len(evt.Points) == 0 {
		return
	}
	writeColor(buf, evt.Red, evt.Green, evt.Blue)
	writeOp(buf, formatNum(evt.Width), "w")
	writePath(buf, evt.Points)
	buf.WriteString("S\n")
}

// writeFill writes one filled path the way the level 0 writer does.
func writeFill(buf *bytes.Buffer, evt Event) {
	if len(evt.Points) == 0 {
		return
	}
	writeColor(buf, evt.Red, evt.Green, evt.Blue)
	writePath(buf, evt.Points)
	opName := "f"
	if evt.EvenOdd {
		opName = "f*"
	}
	writeOp(buf, opName)
}

// writeColor writes both the stroke and the fill color, so one paint operator
// is enough.
func writeColor(buf *bytes.Buffer, red, green, blue float64) {
	writeOp(buf, formatNum(red), formatNum(green), formatNum(blue), "RG")
	writeOp(buf, formatNum(red), formatNum(green), formatNum(blue), "rg")
}

// writePath writes m and l operators.
func writePath(buf *bytes.Buffer, pts []graphics.Point) {
	for _, point := range pts {
		opName := "l"
		if point.Move {
			opName = "m"
		}
		writeOp(buf, formatNum(point.X), formatNum(point.Y), opName)
	}
}

// writeImage writes one Do with the recorded XObject name.
func writeImage(buf *bytes.Buffer, name string) {
	writeOp(buf, nameText(name), "Do")
}

// writeTextRun writes one text object from the recorded run. The absolute Tm
// reproduces the placement, so Td and TD are not needed. Tc, Tw, Tz, and Ts
// are always written because they persist across BT and ET.
func writeTextRun(buf *bytes.Buffer, run TextRun) {
	buf.WriteString("BT\n")
	writeOp(buf, nameText(run.FontName), formatNum(run.Size), "Tf")
	writeOp(buf, formatNum(run.CharSpacing), "Tc")
	writeOp(buf, formatNum(run.WordSpacing), "Tw")
	hscale := run.HScale
	if hscale == 0 {
		hscale = 1
	}
	writeOp(buf, formatNum(hscale*percentFull), "Tz")
	writeOp(buf, formatNum(run.Rise), "Ts")
	writeOp(buf, matrixText(run.TextMatrix), "Tm")
	writeOp(buf, hexText(run.Bytes), "Tj")
	buf.WriteString("ET\n")
}

// matrixText writes one matrix as the six Tm operands.
func matrixText(mat graphics.Matrix) string {
	return formatNum(mat.A) + " " + formatNum(mat.B) + " " + formatNum(mat.C) + " " +
		formatNum(mat.D) + " " + formatNum(mat.E) + " " + formatNum(mat.F)
}

// nameText writes one name with the escapes SerializeValue uses.
func nameText(name string) string {
	return string(pdf.SerializeValue(pdf.NameVal(name)))
}

// hexText writes one string as a hex string, so any byte survives.
func hexText(raw []byte) string {
	const hexPairLen = 2
	const hexDigits = "0123456789ABCDEF"
	out := make([]byte, 0, len(raw)*hexPairLen+hexPairLen)
	out = append(out, '<')
	for _, cur := range raw {
		out = append(out, hexDigits[cur>>4], hexDigits[cur&0x0F])
	}
	out = append(out, '>')
	return string(out)
}

// writeOp writes one operator line.
func writeOp(buf *bytes.Buffer, parts ...string) {
	for index, part := range parts {
		if index > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(part)
	}
	buf.WriteByte('\n')
}

// formatNum prints a decimal without an exponent.
func formatNum(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
