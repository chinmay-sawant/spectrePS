package tag

// This file derives a structure plan from recorded content events: reading
// order, paragraphs, headings, lists, tables, figures, ActualText, and
// artifacts. An untagged PDF stores no semantics, so the reading order is an
// approximation from device geometry, not a fact, open decision 5.

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/chinmay-sawant/spectrePS/internal/graphics"
	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// The geometry thresholds. They are fractions of one run box height unless
// the name says points. documentation/devices.md records them.
const (
	lineTolerance     = 0.5
	rowTolerance      = 0.25
	paragraphGap      = 1.35
	headingStep       = 1.2
	maxHeadingLevel   = 6
	tableTolerance    = 4.0
	tableMinGap       = 2.0
	tableMinCells     = 2
	tableMinRows      = 2
	columnGap         = 3.0
	furnitureTol      = 2.0
	furnitureMaxRunes = 64
	furniturePages    = 2
	maxListDigits     = 3
	fallbackAscent    = 0.75
	fallbackDescent   = 0.25
	fallbackWidth     = 0.5
)

// newElem returns one empty authored element of a type. Every field is set,
// so a later literal does not have to repeat the neutral values.
func newElem(typeName string) *Element {
	return &Element{
		Type:          typeName,
		Alt:           "",
		ActualText:    "",
		Lang:          "",
		Scope:         "",
		ListNumbering: "",
		Kids:          nil,
		Claims:        nil,
	}
}

// flowKind names one ordered item on a page.
type flowKind int

const (
	flowBlock flowKind = iota
	flowTable
	flowFigure
)

// runInfo is one recorded text run with its device geometry and decoded text.
type runInfo struct {
	event     int
	text      string
	actual    string
	hasActual bool
	size      float64
	x         float64
	y         float64
	box       pdf.Box
}

// figureInfo is one recorded image with its alt text and device box.
type figureInfo struct {
	event int
	alt   string
	box   pdf.Box
}

// textLine is one row of runs that share a baseline.
type textLine struct {
	runs       []runInfo
	top        float64
	bottom     float64
	minX       float64
	maxX       float64
	size       float64
	baselineY  float64
	lineHeight float64
}

// textBlock is one or more lines that read as a paragraph.
type textBlock struct {
	lines  []textLine
	top    float64
	bottom float64
	minX   float64
	maxX   float64
	size   float64
	text   string
}

// tableGrid is one inferred table: rows of cell segments with matching
// column starts.
type tableGrid struct {
	rows [][]textLine
}

// flowItem is one ordered item: a block, a table, or a figure.
type flowItem struct {
	kind   flowKind
	top    float64
	bottom float64
	minX   float64
	maxX   float64
	block  *textBlock
	table  *tableGrid
	figure *figureInfo
}

// pageLayout is one page's derived geometry before element authoring.
type pageLayout struct {
	page    int
	rec     *Recorder
	lines   []textLine
	blocks  []*textBlock
	tables  []*tableGrid
	figures []figureInfo
}

// furnitureKey identifies repeated page furniture: the same text in the same
// vertical band.
type furnitureKey struct {
	text string
	top  int
}

// DerivePlan turns recorded pages into an authored structure plan. The plan
// covers every recorded event: a text run joins an element, an image becomes
// a Figure with /Alt, and everything else is wrapped in /Artifact. An image
// with no alt source is refused with /alt in Tag, open decision 4.
func DerivePlan(pages []*Recorder) (*Plan, error) {
	for _, rec := range pages {
		if rec == nil {
			return nil, pdf.NewError(opTag, errPlan)
		}
	}
	layouts := make([]*pageLayout, 0, len(pages))
	for page, rec := range pages {
		layout, err := derivePage(page, rec)
		if err != nil {
			return nil, err
		}
		layouts = append(layouts, layout)
	}
	body := bodySize(pages)
	ranks := headingRanks(pages, body)
	furniture := furnitureKeys(layouts)
	doc := newElem(documentType)
	plan := &Plan{Root: doc, Roles: nil, Artifacts: nil}
	for _, layout := range layouts {
		elems, artifacts := layout.elements(ranks, furniture)
		doc.Kids = append(doc.Kids, elems...)
		plan.Artifacts = append(plan.Artifacts, artifacts...)
	}
	return plan, nil
}

// derivePage reads one recorder into runs, lines, tables, and blocks.
func derivePage(page int, rec *Recorder) (*pageLayout, error) {
	runs, figures, err := pageContent(rec.events)
	if err != nil {
		return nil, err
	}
	layout := &pageLayout{
		page:    page,
		rec:     rec,
		lines:   nil,
		blocks:  nil,
		tables:  nil,
		figures: figures,
	}
	layout.lines = buildLines(runs)
	rest := make([]textLine, 0, len(layout.lines))
	for index := 0; index < len(layout.lines); {
		grid, next := matchTable(layout.lines, index)
		if grid != nil {
			layout.tables = append(layout.tables, grid)
			index = next
			continue
		}
		rest = append(rest, layout.lines[index])
		index++
	}
	layout.blocks = buildBlocks(rest)
	return layout, nil
}

// pageContent walks one page in stream order and pairs every image XObject
// with its alt text. The innermost open BDC property dictionary wins, then
// the image XObject dictionary itself, open decision 4.
func pageContent(events []Event) ([]runInfo, []figureInfo, error) {
	runs := make([]runInfo, 0, len(events))
	figures := make([]figureInfo, 0, len(events))
	props := make([]pdf.Value, 0, structDepthCap)
	for index, evt := range events {
		switch evt.Kind {
		case EventBeginMarked:
			props = append(props, evt.Properties)
		case EventEndMarked:
			if len(props) > 0 {
				props = props[:len(props)-1]
			}
		case EventText:
			info, ok := runFromEvent(index, evt.Text)
			if ok {
				runs = append(runs, info)
			}
		case EventImage:
			alt := imageAlt(props, evt.ImageDict)
			if alt == "" {
				return nil, nil, pdf.NewError(opTag, errAlt)
			}
			figures = append(figures, figureInfo{event: index, alt: alt, box: evt.ImageBox})
		case EventStroke, EventFill:
			// Paths become artifacts unless an element claims them; no
			// element claims a path today.
		}
	}
	return runs, figures, nil
}

// imageAlt returns the first /Alt string in the open property dictionaries,
// then the /Alt of the image dictionary.
func imageAlt(props []pdf.Value, dict pdf.Value) string {
	for index := len(props) - 1; index >= 0; index-- {
		if alt := altEntry(props[index]); alt != "" {
			return alt
		}
	}
	return altEntry(dict)
}

// altEntry reads one /Alt string entry.
func altEntry(val pdf.Value) string {
	entry, ok := val.ValueEntry(keyAlt)
	if !ok || entry.Kind != pdf.KindString {
		return ""
	}
	return entry.String
}

// runFromEvent decodes one text event. A run that decodes to no text is
// skipped, and its event becomes an artifact.
func runFromEvent(index int, run pdf.TextRun) (runInfo, bool) {
	text := runText(run)
	if text == "" {
		return zeroRunInfo(), false
	}
	actual, needs := actualTextOf(run)
	posX, posY := deviceOrigin(run)
	return runInfo{
		event:     index,
		text:      text,
		actual:    actual,
		hasActual: needs,
		size:      run.Size,
		x:         posX,
		y:         posY,
		box:       runBox(run, text),
	}, true
}

// runText decodes one run with the /ToUnicode mapping and the code-point
// fallback internal/pdf extraction uses.
func runText(run pdf.TextRun) string {
	var out strings.Builder
	for index, code := range run.Codes {
		unit := unicodeAt(run, index)
		if unit == "" {
			unit = codePointText(code)
		}
		out.WriteString(unit)
	}
	return out.String()
}

// zeroRunInfo returns the empty run.
func zeroRunInfo() runInfo {
	return runInfo{
		event: 0, text: "", actual: "", hasActual: false, size: 0, x: 0, y: 0,
		box: pdf.Box{MinX: 0, MinY: 0, MaxX: 0, MaxY: 0},
	}
}

// unicodeAt returns the /ToUnicode result at one index, empty when absent.
func unicodeAt(run pdf.TextRun, index int) string {
	if index < 0 || index >= len(run.Unicode) {
		return ""
	}
	return run.Unicode[index]
}

// actualTextOf applies the narrow ActualText policy, open decision 9: only a
// ligature code point or a code with no Unicode mapping needs /ActualText. A
// ligature expands to its letters and an unmapped code falls back to the code
// point, the same text extraction produces.
func actualTextOf(run pdf.TextRun) (string, bool) {
	var out strings.Builder
	needs := false
	for index, code := range run.Codes {
		unit := unicodeAt(run, index)
		if unit == "" {
			needs = true
			out.WriteString(codePointText(code))
			continue
		}
		if expanded, ok := ligatureText(unit); ok {
			needs = true
			out.WriteString(expanded)
			continue
		}
		out.WriteString(unit)
	}
	return out.String(), needs
}

// ligatureText expands the narrow ligature set: the ligature code points
// /ActualText replaces.
func ligatureText(unit string) (string, bool) {
	switch unit {
	case "\uFB00":
		return "ff", true
	case "\uFB01":
		return "fi", true
	case "\uFB02":
		return "fl", true
	case "\uFB03":
		return "ffi", true
	case "\uFB04":
		return "ffl", true
	case "\uFB05", "\uFB06":
		return "st", true
	case "\u0132":
		return "IJ", true
	case "\u0133":
		return "ij", true
	default:
		return "", false
	}
}

// codePointText is the fallback for a font with no Unicode mapping: a
// printable code point stands for itself.
func codePointText(code uint32) string {
	if code < 0x20 || code > unicode.MaxRune {
		return ""
	}
	if code >= 0xD800 && code <= 0xDFFF {
		return ""
	}
	return string(rune(code))
}

// deviceOrigin is the device-space baseline origin of one run.
func deviceOrigin(run pdf.TextRun) (float64, float64) {
	return runDeviceMatrix(run).Apply(0, 0)
}

// runDeviceMatrix maps text space to device pixels, the same matrix the text
// machine paints with.
func runDeviceMatrix(run pdf.TextRun) graphics.Matrix {
	ctm := run.CTM
	if ctm == (graphics.Matrix{A: 0, B: 0, C: 0, D: 0, E: 0, F: 0}) {
		ctm = graphics.Identity()
	}
	scale := run.Scale
	if scale == 0 {
		scale = 1
	}
	deviceScale := graphics.Matrix{A: scale, B: 0, C: 0, D: scale, E: 0, F: 0}
	return graphics.Concat(run.TextMatrix, graphics.Concat(ctm, deviceScale))
}

// runBox returns the recorded device box, or one synthesized from the size
// and the decoded text when a hand-built event carries none.
func runBox(run pdf.TextRun, text string) pdf.Box {
	box := run.Box
	if box.MaxX > box.MinX && box.MaxY > box.MinY {
		return box
	}
	posX, posY := deviceOrigin(run)
	width := float64(utf8.RuneCountInString(text)) * run.Size * fallbackWidth
	if width <= 0 {
		width = run.Size
	}
	height := run.Size
	return pdf.Box{
		MinX: posX,
		MinY: posY - height*fallbackDescent,
		MaxX: posX + width,
		MaxY: posY + height*fallbackAscent,
	}
}

// buildLines clusters runs by baseline and splits each baseline into one
// segment per wide-gap group. A column gutter becomes a separate segment, so
// block grouping sees columns and a table row reads as one segment per cell.
func buildLines(runs []runInfo) []textLine {
	sorted := make([]runInfo, len(runs))
	copy(sorted, runs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].y != sorted[j].y {
			return sorted[i].y > sorted[j].y
		}
		return sorted[i].box.MinX < sorted[j].box.MinX
	})
	clusters := make([][]runInfo, 0, len(sorted))
	for _, run := range sorted {
		if len(clusters) == 0 || !sameBaselineCluster(clusters[len(clusters)-1], run) {
			clusters = append(clusters, []runInfo{run})
			continue
		}
		clusters[len(clusters)-1] = append(clusters[len(clusters)-1], run)
	}
	lines := make([]textLine, 0, len(sorted))
	for _, cluster := range clusters {
		sort.SliceStable(cluster, func(i, j int) bool {
			return cluster[i].box.MinX < cluster[j].box.MinX
		})
		current := newLine(cluster[0])
		for _, run := range cluster[1:] {
			if nearLine(current, run) {
				current = appendRun(current, run)
				continue
			}
			lines = append(lines, current)
			current = newLine(run)
		}
		lines = append(lines, current)
	}
	return lines
}

// sameBaselineCluster reports whether a run joins the current baseline
// cluster. The cluster keeps its first baseline, so a drifting baseline does
// not walk away from the line.
func sameBaselineCluster(cluster []runInfo, run runInfo) bool {
	base := cluster[0]
	height := 0.0
	for _, member := range cluster {
		height = math.Max(height, lineHeight(member))
	}
	if height <= 0 {
		height = 1
	}
	return math.Abs(run.y-base.y) <= lineTolerance*height
}

// newLine starts one line at a baseline.
func newLine(run runInfo) textLine {
	return textLine{
		runs:       []runInfo{run},
		top:        run.box.MaxY,
		bottom:     run.box.MinY,
		minX:       run.box.MinX,
		maxX:       run.box.MaxX,
		size:       run.size,
		baselineY:  run.y,
		lineHeight: lineHeight(run),
	}
}

// appendRun adds one run to a line and grows its bounds.
func appendRun(line textLine, run runInfo) textLine {
	line.runs = append(line.runs, run)
	line.top = math.Max(line.top, run.box.MaxY)
	line.bottom = math.Min(line.bottom, run.box.MinY)
	line.minX = math.Min(line.minX, run.box.MinX)
	line.maxX = math.Max(line.maxX, run.box.MaxX)
	line.size = math.Max(line.size, run.size)
	line.lineHeight = math.Max(line.lineHeight, lineHeight(run))
	return line
}

// lineHeight is one run's device box height, with the size as a fallback.
func lineHeight(run runInfo) float64 {
	height := run.box.MaxY - run.box.MinY
	if height > 0 {
		return height
	}
	if run.size > 0 {
		return run.size
	}
	return 1
}

// nearLine reports whether a run continues the line horizontally. A gap
// wider than the column threshold starts a new segment, so a two-column page
// does not read as one row.
func nearLine(line textLine, run runInfo) bool {
	return run.box.MinX-line.maxX <= columnGap*line.lineHeight
}

// matchTable reports whether one baseline row starts a table and returns the
// rows it covers. A table row is two or more well-spread cells, and the rows
// below match the same columns.
func matchTable(lines []textLine, start int) (*tableGrid, int) {
	row, next := sameRow(lines, start)
	if len(row) < tableMinCells {
		return nil, start
	}
	columns := rowColumns(row)
	if !columnsSpread(columns, rowHeight(row)) {
		return nil, start
	}
	rows := [][]textLine{row}
	index := next
	for index < len(lines) {
		nextRow, after := sameRow(lines, index)
		if len(nextRow) != len(row) || !matchColumns(nextRow, columns) {
			break
		}
		rows = append(rows, nextRow)
		index = after
	}
	if len(rows) < tableMinRows {
		return nil, start
	}
	return &tableGrid{rows: rows}, index
}

// sameRow collects the consecutive segments that share one baseline.
func sameRow(lines []textLine, start int) ([]textLine, int) {
	row := []textLine{lines[start]}
	index := start + 1
	for index < len(lines) && sameRowBaseline(lines[start], lines[index]) {
		row = append(row, lines[index])
		index++
	}
	return row, index
}

// sameRowBaseline reports whether two segments share a row baseline. A table
// row is written at one baseline, so the tolerance is tighter than the
// paragraph line tolerance.
func sameRowBaseline(first, second textLine) bool {
	height := math.Max(first.lineHeight, second.lineHeight)
	if height <= 0 {
		height = 1
	}
	return math.Abs(first.baselineY-second.baselineY) <= rowTolerance*height
}

// rowColumns returns one row's cell starts, sorted left to right.
func rowColumns(row []textLine) []float64 {
	out := make([]float64, 0, len(row))
	for _, cell := range row {
		out = append(out, cell.minX)
	}
	sort.Float64s(out)
	return out
}

// rowHeight is the tallest cell height in one row.
func rowHeight(row []textLine) float64 {
	height := 0.0
	for _, cell := range row {
		height = math.Max(height, cell.lineHeight)
	}
	if height <= 0 {
		return 1
	}
	return height
}

// columnsSpread reports whether every column start is at least the minimum
// gap away from the one before. A tight prefix and body, as in a list, is not
// a table.
func columnsSpread(columns []float64, height float64) bool {
	minGap := tableMinGap * height
	for index := 1; index < len(columns); index++ {
		if columns[index]-columns[index-1] < minGap {
			return false
		}
	}
	return true
}

// matchColumns reports whether one row fills the same columns.
func matchColumns(row []textLine, columns []float64) bool {
	if len(row) != len(columns) {
		return false
	}
	starts := rowColumns(row)
	for index := range columns {
		if math.Abs(starts[index]-columns[index]) > tableTolerance {
			return false
		}
	}
	return true
}

// buildBlocks merges lines into paragraphs. Two lines belong to one block
// when their boxes overlap horizontally and their vertical gap is under the
// paragraph threshold.
func buildBlocks(lines []textLine) []*textBlock {
	sorted := make([]textLine, len(lines))
	copy(sorted, lines)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].top != sorted[j].top {
			return sorted[i].top > sorted[j].top
		}
		return sorted[i].minX < sorted[j].minX
	})
	blocks := make([]*textBlock, 0, len(sorted))
	for _, line := range sorted {
		best := -1
		bestGap := math.Inf(1)
		for index, block := range blocks {
			gap, ok := blockFits(block, line)
			if ok && gap < bestGap {
				best = index
				bestGap = gap
			}
		}
		if best < 0 {
			blocks = append(blocks, newBlock(line))
			continue
		}
		appendLine(blocks[best], line)
	}
	for _, block := range blocks {
		block.text = blockText(block)
	}
	return blocks
}

// blockFits reports whether one line continues a block, with the vertical
// gap as the tie-break. The threshold follows the smaller of the block and
// the line, so a heading does not swallow the text below it.
func blockFits(block *textBlock, line textLine) (float64, bool) {
	if line.maxX <= block.minX || line.minX >= block.maxX {
		return 0, false
	}
	gap := block.bottom - line.top
	if gap < 0 {
		gap = line.bottom - block.top
	}
	height := math.Min(blockHeight(block), line.lineHeight)
	if gap > paragraphGap*height {
		return 0, false
	}
	return gap, true
}

// newBlock starts a block from one line.
func newBlock(line textLine) *textBlock {
	return &textBlock{
		lines:  []textLine{line},
		top:    line.top,
		bottom: line.bottom,
		minX:   line.minX,
		maxX:   line.maxX,
		size:   line.size,
		text:   "",
	}
}

// appendLine adds one line to a block and grows its bounds.
func appendLine(block *textBlock, line textLine) {
	block.lines = append(block.lines, line)
	block.top = math.Max(block.top, line.top)
	block.bottom = math.Min(block.bottom, line.bottom)
	block.minX = math.Min(block.minX, line.minX)
	block.maxX = math.Max(block.maxX, line.maxX)
	block.size = math.Max(block.size, line.size)
}

// blockHeight is the tallest line height in one block.
func blockHeight(block *textBlock) float64 {
	height := 0.0
	for _, line := range block.lines {
		height = math.Max(height, line.lineHeight)
	}
	if height <= 0 {
		return 1
	}
	return height
}

// blockText joins the line texts.
func blockText(block *textBlock) string {
	parts := make([]string, 0, len(block.lines))
	for _, line := range block.lines {
		parts = append(parts, lineText(line))
	}
	return strings.Join(parts, " ")
}

// lineText joins one line's run texts with a space at each boundary.
func lineText(line textLine) string {
	parts := make([]string, 0, len(line.runs))
	for _, run := range line.runs {
		parts = append(parts, run.text)
	}
	return strings.Join(parts, " ")
}

// runsInEventOrder returns the block runs in stream order.
func (block *textBlock) runsInEventOrder() []runInfo {
	out := make([]runInfo, 0, len(block.lines))
	for _, line := range block.lines {
		out = append(out, line.runs...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].event < out[j].event
	})
	return out
}

// bodySize is the decoded-text weighted mode of the run sizes across the
// document. It is the paragraph size a heading steps above.
func bodySize(pages []*Recorder) float64 {
	weights := map[float64]int{}
	for _, rec := range pages {
		for _, evt := range rec.events {
			if evt.Kind != EventText {
				continue
			}
			weights[evt.Text.Size] += utf8.RuneCountInString(runText(evt.Text))
		}
	}
	best := 0.0
	bestWeight := 0
	for size, weight := range weights {
		if weight > bestWeight || (weight == bestWeight && size > best) {
			best = size
			bestWeight = weight
		}
	}
	return best
}

// headingRanks maps every run size a clear step above the body size to H1
// through H6. The largest size is H1.
func headingRanks(pages []*Recorder, body float64) map[float64]string {
	ranks := map[float64]string{}
	if body <= 0 {
		return ranks
	}
	sizes := map[float64]bool{}
	for _, rec := range pages {
		for _, evt := range rec.events {
			if evt.Kind == EventText && evt.Text.Size >= body*headingStep {
				sizes[evt.Text.Size] = true
			}
		}
	}
	ordered := make([]float64, 0, len(sizes))
	for size := range sizes {
		ordered = append(ordered, size)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(ordered)))
	for index, size := range ordered {
		level := index + 1
		if level > maxHeadingLevel {
			level = maxHeadingLevel
		}
		ranks[size] = "H" + strconv.Itoa(level)
	}
	return ranks
}

// furnitureKeys returns the blocks that repeat at the same vertical band on
// two or more pages. Repeated furniture stays out of the reading order.
func furnitureKeys(layouts []*pageLayout) map[furnitureKey]bool {
	pages := map[furnitureKey]map[int]bool{}
	for _, layout := range layouts {
		for _, block := range layout.blocks {
			key, ok := furnitureOf(block)
			if !ok {
				continue
			}
			if pages[key] == nil {
				pages[key] = map[int]bool{}
			}
			pages[key][layout.page] = true
		}
	}
	repeated := map[furnitureKey]bool{}
	for key, seen := range pages {
		if len(seen) >= furniturePages {
			repeated[key] = true
		}
	}
	return repeated
}

// furnitureOf returns the furniture key of one block, or false when the block
// is too long to be furniture.
func furnitureOf(block *textBlock) (furnitureKey, bool) {
	text := strings.TrimSpace(block.text)
	if text == "" || utf8.RuneCountInString(text) > furnitureMaxRunes {
		return furnitureKey{text: "", top: 0}, false
	}
	return furnitureKey{text: text, top: int(math.Round(block.top / furnitureTol))}, true
}

// isFurniture reports whether one block repeats across pages.
func isFurniture(block *textBlock, repeated map[furnitureKey]bool) bool {
	key, ok := furnitureOf(block)
	return ok && repeated[key]
}

// elements authors the page elements and the artifact spans. Every event is
// covered: a text run joins an element, a figure claims its image, and the
// rest is wrapped in /Artifact.
func (layout *pageLayout) elements(
	ranks map[float64]string,
	furniture map[furnitureKey]bool,
) ([]*Element, []Claim) {
	items := layout.flowItems(furniture)
	items = orderFlow(items)
	elems := make([]*Element, 0, len(items))
	for index := range items {
		item := &items[index]
		switch item.kind {
		case flowTable:
			elems = append(elems, tableElement(layout.page, item.table))
		case flowFigure:
			elems = append(elems, figureElement(layout.page, item.figure))
		case flowBlock:
			elems = append(elems, layout.blockElement(ranks, item.block))
		}
	}
	claimed := claimIndex(elems)
	artifacts := layout.artifactSpans(claimed)
	return elems, artifacts
}

// flowItems turns the page geometry into ordered candidates. A furniture
// block stays out of the candidates, so its events become artifacts.
func (layout *pageLayout) flowItems(furniture map[furnitureKey]bool) []flowItem {
	items := make([]flowItem, 0, len(layout.blocks)+len(layout.tables)+len(layout.figures))
	for _, block := range layout.blocks {
		if isFurniture(block, furniture) {
			continue
		}
		items = append(items, flowItem{
			kind: flowBlock, top: block.top, bottom: block.bottom,
			minX: block.minX, maxX: block.maxX,
			block: block, table: nil, figure: nil,
		})
	}
	for _, grid := range layout.tables {
		top, bottom, minX, maxX := gridBounds(grid)
		items = append(items, flowItem{
			kind: flowTable, top: top, bottom: bottom,
			minX: minX, maxX: maxX,
			block: nil, table: grid, figure: nil,
		})
	}
	for index := range layout.figures {
		figure := &layout.figures[index]
		items = append(items, flowItem{
			kind: flowFigure, top: figure.box.MaxY, bottom: figure.box.MinY,
			minX: figure.box.MinX, maxX: figure.box.MaxX,
			block: nil, table: nil, figure: figure,
		})
	}
	return items
}

// gridBounds returns the union box of one table grid.
func gridBounds(grid *tableGrid) (float64, float64, float64, float64) {
	top, bottom := math.Inf(-1), math.Inf(1)
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, row := range grid.rows {
		for _, cell := range row {
			top = math.Max(top, cell.top)
			bottom = math.Min(bottom, cell.bottom)
			minX = math.Min(minX, cell.minX)
			maxX = math.Max(maxX, cell.maxX)
		}
	}
	return top, bottom, minX, maxX
}

// orderFlow sorts the page items into reading order. Items that overlap
// vertically order left to right, everything else top to bottom.
func orderFlow(items []flowItem) []flowItem {
	remaining := make([]flowItem, len(items))
	copy(remaining, items)
	out := make([]flowItem, 0, len(remaining))
	for len(remaining) > 0 {
		best := 0
		for index := 1; index < len(remaining); index++ {
			if before(remaining[index], remaining[best]) {
				best = index
			}
		}
		out = append(out, remaining[best])
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return out
}

// before reports whether one item reads before another.
func before(left, right flowItem) bool {
	if verticalOverlap(left, right) > 0 && left.minX != right.minX {
		return left.minX < right.minX
	}
	if left.top != right.top {
		return left.top > right.top
	}
	return left.minX < right.minX
}

// verticalOverlap is the shared vertical extent of two items.
func verticalOverlap(left, right flowItem) float64 {
	return math.Min(left.top, right.top) - math.Max(left.bottom, right.bottom)
}

// blockElement authors one block as a heading, a list, or a paragraph.
func (layout *pageLayout) blockElement(ranks map[float64]string, block *textBlock) *Element {
	if level, ok := ranks[block.size]; ok {
		elem := newElem(level)
		addRunText(layout.page, elem, block.runsInEventOrder())
		return elem
	}
	if blockIsList(block) {
		return listElement(layout.page, block)
	}
	elem := newElem("P")
	addRunText(layout.page, elem, block.runsInEventOrder())
	return elem
}

// blockIsList reports whether every line of one block carries a list prefix.
func blockIsList(block *textBlock) bool {
	if len(block.lines) == 0 {
		return false
	}
	for _, line := range block.lines {
		if _, ok := listPrefix(lineText(line)); !ok {
			return false
		}
	}
	return true
}

// listElement authors one list: /L with one /LI per line, each /LI carrying
// /Lbl for the prefix and /LBody for the rest. The /L element carries the
// /ListNumbering attribute PDF/UA-2 requires beside a label.
func listElement(page int, block *textBlock) *Element {
	list := newElem("L")
	for index, line := range block.lines {
		text := lineText(line)
		prefix, ok := listPrefix(text)
		if !ok {
			continue
		}
		if index == 0 {
			list.ListNumbering = listNumbering(prefix)
		}
		label := newElem("Lbl")
		body := newElem("LBody")
		head, tail := splitPrefix(line.runs, utf8.RuneCountInString(prefix))
		addRunText(page, label, head)
		addRunText(page, body, tail)
		item := newElem("LI")
		item.Kids = []*Element{label, body}
		list.Kids = append(list.Kids, item)
	}
	return list
}

// listNumbering maps one prefix to the /ListNumbering value: Decimal for a
// number and Disc for a bullet.
func listNumbering(prefix string) string {
	if prefix != "" && prefix[0] >= '0' && prefix[0] <= '9' {
		return "Decimal"
	}
	return "Disc"
}

// splitPrefix splits one line's runs at a rune count. A run the prefix ends
// inside stays in the body, because a claim cannot split one event.
func splitPrefix(runs []runInfo, prefixLen int) ([]runInfo, []runInfo) {
	var head, tail []runInfo
	used := 0
	for _, run := range runs {
		runLen := utf8.RuneCountInString(run.text)
		if used >= prefixLen {
			tail = append(tail, run)
			continue
		}
		if used+runLen <= prefixLen {
			head = append(head, run)
		} else {
			tail = append(tail, run)
		}
		used += runLen
	}
	return head, tail
}

// listPrefix matches a bullet or number prefix and returns it.
func listPrefix(text string) (string, bool) {
	if prefix, ok := bulletPrefix(text); ok {
		return prefix, true
	}
	return numberPrefix(text)
}

// bulletMarker is one accepted bullet character.
type bulletMarker struct {
	text       string
	needsSpace bool
}

// bulletMarks is the narrow bullet set. A hyphen or asterisk needs a space,
// so a negative number or a glob is not a list.
func bulletMarks() []bulletMarker {
	return []bulletMarker{
		{text: "\u2022", needsSpace: false},
		{text: "\u00B7", needsSpace: false},
		{text: "\u25E6", needsSpace: false},
		{text: "\u2023", needsSpace: false},
		{text: "\u25AA", needsSpace: false},
		{text: "\u2013", needsSpace: true},
		{text: "\u2014", needsSpace: true},
		{text: "-", needsSpace: true},
		{text: "*", needsSpace: true},
	}
}

// bulletPrefix matches one bullet mark.
func bulletPrefix(text string) (string, bool) {
	for _, mark := range bulletMarks() {
		if !strings.HasPrefix(text, mark.text) {
			continue
		}
		rest := text[len(mark.text):]
		if rest != "" && !strings.HasPrefix(rest, " ") {
			continue
		}
		if rest == "" && mark.needsSpace {
			continue
		}
		return mark.text, true
	}
	return "", false
}

// numberPrefix matches a decimal number followed by a period or a close
// parenthesis and a space or the end of the text. A second number, as in
// "1.2 Results", is not a list prefix.
func numberPrefix(text string) (string, bool) {
	digits := digitCount(text)
	if digits == 0 || digits >= len(text) {
		return "", false
	}
	mark := text[digits]
	if mark != '.' && mark != ')' {
		return "", false
	}
	rest := text[digits+1:]
	if rest == "" {
		return text[:digits+1], true
	}
	if !strings.HasPrefix(rest, " ") {
		return "", false
	}
	return text[:digits+1], true
}

// digitCount counts the leading decimal digits, up to three.
func digitCount(text string) int {
	count := 0
	for count < len(text) && count < maxListDigits && text[count] >= '0' && text[count] <= '9' {
		count++
	}
	return count
}

// tableElement authors one table. The first row carries /TH cells with
// /Scope /Column and the rest carry /TD cells.
func tableElement(page int, grid *tableGrid) *Element {
	table := newElem("Table")
	for index, gridRow := range grid.rows {
		row := newElem("TR")
		for _, cellLine := range gridRow {
			cell := newElem("TD")
			if index == 0 {
				cell.Type = "TH"
				cell.Scope = "Column"
			}
			addRunText(page, cell, cellLine.runs)
			row.Kids = append(row.Kids, cell)
		}
		table.Kids = append(table.Kids, row)
	}
	return table
}

// figureElement authors one image as a Figure with its /Alt.
func figureElement(page int, figure *figureInfo) *Element {
	elem := newElem("Figure")
	elem.Alt = figure.alt
	elem.Claims = []Claim{{Page: page, First: figure.event, Last: figure.event + 1}}
	return elem
}

// addRunText adds the claims of one element's runs. Adjacent runs merge into
// one claim, and a run that needs ActualText becomes its own claim with the
// text on the element, or a /Span child when the element carries other runs
// too.
func addRunText(page int, elem *Element, runs []runInfo) {
	if len(runs) == 0 {
		return
	}
	if len(runs) == 1 && runs[0].hasActual {
		elem.ActualText = runs[0].actual
		elem.Claims = append(elem.Claims, runClaim(page, runs[0]))
		return
	}
	order := make([]runInfo, len(runs))
	copy(order, runs)
	sort.SliceStable(order, func(i, j int) bool {
		return order[i].event < order[j].event
	})
	first, last := -1, -1
	flush := func() {
		if first >= 0 {
			elem.Claims = append(elem.Claims, Claim{Page: page, First: first, Last: last})
			first, last = -1, -1
		}
	}
	for _, run := range order {
		if run.hasActual {
			flush()
			span := newElem("Span")
			span.ActualText = run.actual
			span.Claims = []Claim{runClaim(page, run)}
			elem.Kids = append(elem.Kids, span)
			continue
		}
		if first >= 0 && run.event != last {
			flush()
		}
		if first < 0 {
			first = run.event
		}
		last = run.event + 1
	}
	flush()
}

// runClaim is one claim for a single run event.
func runClaim(page int, run runInfo) Claim {
	return Claim{Page: page, First: run.event, Last: run.event + 1}
}

// claimIndex returns every event index the elements and their children claim.
func claimIndex(elems []*Element) map[int]bool {
	claimed := map[int]bool{}
	var walk func(elem *Element)
	walk = func(elem *Element) {
		for _, claim := range elem.Claims {
			for index := claim.First; index < claim.Last; index++ {
				claimed[index] = true
			}
		}
		for _, kid := range elem.Kids {
			walk(kid)
		}
	}
	for _, elem := range elems {
		walk(elem)
	}
	return claimed
}

// artifactSpans wraps every unclaimed event in /Artifact BMC. Source
// marked-content events are dropped by the writer, so they are skipped here.
func (layout *pageLayout) artifactSpans(claimed map[int]bool) []Claim {
	artifacts := make([]Claim, 0, len(layout.rec.events))
	start := -1
	for index, evt := range layout.rec.events {
		covered := claimed[index] || evt.marked()
		if !covered && start < 0 {
			start = index
		}
		if covered && start >= 0 {
			artifacts = append(artifacts, Claim{Page: layout.page, First: start, Last: index})
			start = -1
		}
	}
	if start >= 0 {
		artifacts = append(artifacts, Claim{Page: layout.page, First: start, Last: len(layout.rec.events)})
	}
	return artifacts
}
