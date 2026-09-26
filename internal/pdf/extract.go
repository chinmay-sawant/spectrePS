package pdf

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Layout thresholds as fractions of one glyph box height.
const (
	lineTolerance  = 0.5
	spaceTolerance = 0.25
)

// ExtractText returns the text of a zero-based page. Lines run top to bottom
// and left to right, each line ends with CRLF, and a font with neither
// /ToUnicode nor a named encoding falls back to the code point.
// A bad index is rangecheck. A nil context panics.
func (file *File) ExtractText(ctx context.Context, index int) (string, error) {
	if ctx == nil {
		panic(panicNilCtx)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	content, err := file.Content(index)
	if err != nil {
		return "", err
	}
	res, err := file.PageResources(index)
	if err != nil {
		return "", err
	}
	collector := &textCollector{glyphs: nil}
	opt := PaintOptions{Resources: res, Text: TextOptions{Fonts: nil, Sink: collector, Runs: nil}, MarkedContent: nil}
	if err := PaintWith(ctx, content, nil, 1, opt); err != nil {
		return "", err
	}
	return layoutGlyphs(collector.glyphs), nil
}

// textCollector is the extraction sink. A nil marker with this sink shows
// text without an outline program.
type textCollector struct {
	glyphs []Glyph
}

func (collector *textCollector) Glyph(g Glyph) {
	collector.glyphs = append(collector.glyphs, g)
}

// textItem is one glyph with the text it contributes.
type textItem struct {
	text  string
	glyph Glyph
}

// layoutGlyphs turns positioned glyphs into reading-order text. Every line
// ends with CRLF, following the txtwrite default.
func layoutGlyphs(glyphs []Glyph) string {
	lines := groupLines(textItems(glyphs))
	var out strings.Builder
	for _, line := range lines {
		text := lineText(line)
		if text == "" {
			continue
		}
		out.WriteString(text)
		out.WriteString("\r\n")
	}
	return out.String()
}

// textItems keeps the glyphs that carry text and applies the code-point
// fallback.
func textItems(glyphs []Glyph) []textItem {
	items := make([]textItem, 0, len(glyphs))
	for _, glyph := range glyphs {
		text := glyph.Unicode
		if text == "" {
			text = codePointText(glyph.Code)
		}
		if text == "" {
			continue
		}
		items = append(items, textItem{text: text, glyph: glyph})
	}
	return items
}

// codePointText is the fallback for a font with no /ToUnicode and no named
// encoding: a printable code point stands for itself.
func codePointText(code uint32) string {
	if code < 0x20 || code > unicode.MaxRune {
		return ""
	}
	if code >= 0xD800 && code <= 0xDFFF {
		return ""
	}
	return string(rune(code))
}

// groupLines sorts items top to bottom then left to right and groups them by
// baseline. Each line is sorted left to right again, because items on one
// line can arrive with slightly different baselines.
func groupLines(items []textItem) [][]textItem {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].glyph.Y != items[j].glyph.Y {
			return items[i].glyph.Y > items[j].glyph.Y
		}
		return items[i].glyph.X < items[j].glyph.X
	})
	lines := make([][]textItem, 0)
	for _, item := range items {
		if len(lines) == 0 || !sameLine(lines[len(lines)-1], item) {
			lines = append(lines, []textItem{item})
			continue
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], item)
	}
	for _, line := range lines {
		sort.SliceStable(line, func(i, j int) bool {
			return line[i].glyph.X < line[j].glyph.X
		})
	}
	return lines
}

// sameLine reports whether the item shares the baseline of one line.
func sameLine(line []textItem, item textItem) bool {
	last := line[len(line)-1]
	return math.Abs(item.glyph.Y-last.glyph.Y) <= lineTolerance*boxHeight(last.glyph)
}

func boxHeight(glyph Glyph) float64 {
	height := glyph.Box.MaxY - glyph.Box.MinY
	if height <= 0 {
		return 1
	}
	return height
}

// lineText joins the items of one line and inserts a space where the gap
// between two boxes is wider than the space threshold.
func lineText(line []textItem) string {
	var out strings.Builder
	spaceEnded := true
	for index, item := range line {
		if index > 0 && !spaceEnded && wideGap(line[index-1], item) {
			out.WriteString(" ")
		}
		out.WriteString(item.text)
		spaceEnded = strings.HasSuffix(item.text, " ")
	}
	return strings.TrimRight(out.String(), " ")
}

// wideGap reports whether the box gap between two items is wide enough to
// stand for a missing space.
func wideGap(prev, item textItem) bool {
	gap := item.glyph.Box.MinX - prev.glyph.Box.MaxX
	if gap <= 0 {
		return false
	}
	return gap > spaceTolerance*boxHeight(prev.glyph)
}
