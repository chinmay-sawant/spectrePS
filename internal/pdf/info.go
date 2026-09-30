package pdf

import (
	"slices"
	"strconv"
	"strings"
)

const (
	opInfo = "Info"

	keyMediaBox = "MediaBox"

	subtypeType3        = "Type3"
	subtypeCIDFontType0 = "CIDFontType0"

	infoDefaultPageWidth  = 612
	infoDefaultPageHeight = 792
	infoUnknownFont       = "(none)"
)

// PageSize is one resolved page /MediaBox in points.
type PageSize struct {
	Width  float64
	Height float64
}

// FontInfo is one font dictionary: its /BaseFont name and whether an outline
// program is in the file.
type FontInfo struct {
	Name     string
	Embedded bool
}

// InfoReport is the read-only summary behind spectreps info.
type InfoReport struct {
	Version   string
	Pages     int
	PageSizes []PageSize
	Tagged    bool
	Fonts     []FontInfo
	Images    int
}

// Info reads the document summary. It resolves objects and writes nothing.
// A page with no /MediaBox in its tree reports the reader default of 612 by
// 792 points. Fonts come from every in-use /Type /Font object, so an unused
// font still appears. A null file is typecheck.
func (file *File) Info() (InfoReport, error) {
	if file == nil {
		return InfoReport{}, NewError(opInfo, errType)
	}
	version, err := file.version()
	if err != nil {
		return InfoReport{}, err
	}
	sizes, err := file.pageSizes()
	if err != nil {
		return InfoReport{}, err
	}
	fonts := file.fontInfos()
	images, err := file.ImageObjectNums()
	if err != nil {
		return InfoReport{}, err
	}
	return InfoReport{
		Version:   version,
		Pages:     len(sizes),
		PageSizes: sizes,
		Tagged:    file.HasStructTree(),
		Fonts:     fonts,
		Images:    len(images),
	}, nil
}

// version returns the effective PDF version. A catalog /Version name can
// increase the header version, including after an incremental update.
func (file *File) version() (string, error) {
	headerVersion := file.headerVersion()
	if headerVersion == "" {
		return "", nil
	}
	catalog, err := file.catalogValue()
	if err != nil {
		return "", err
	}
	value, ok := catalog.ValueEntry("Version")
	if !ok || value.Kind != KindName {
		// A /Version written as a number is a producer mistake the header
		// still answers, so the header version stands. pdfTeX and luaTeX both
		// write it that way and Ghostscript reads the file.
		return headerVersion, nil
	}
	catalogMajor, catalogMinor, catalogOK := parsePDFVersion(value.Name)
	if !catalogOK {
		return "", NewError(opInfo, errSyntax)
	}
	headerMajor, headerMinor, headerOK := parsePDFVersion(headerVersion)
	if !headerOK || catalogMajor > headerMajor ||
		(catalogMajor == headerMajor && catalogMinor > headerMinor) {
		return value.Name, nil
	}
	return headerVersion, nil
}

func (file *File) headerVersion() string {
	header := file.Header()
	end := len(header)
	for i, cur := range header {
		if cur == '\n' || cur == '\r' {
			end = i
			break
		}
	}
	line := string(header[:end])
	if !strings.HasPrefix(line, pdfHeader) {
		return ""
	}
	text := line[len(pdfHeader):]
	digits := 0
	for digits < len(text) && (text[digits] == '.' || (text[digits] >= '0' && text[digits] <= '9')) {
		digits++
	}
	return text[:digits]
}

func parsePDFVersion(version string) (int, int, bool) {
	majorText, minorText, ok := strings.Cut(version, ".")
	if !ok || majorText == "" || minorText == "" {
		return 0, 0, false
	}
	major, majorErr := strconv.Atoi(majorText)
	minor, minorErr := strconv.Atoi(minorText)
	if majorErr != nil || minorErr != nil || major < 0 || minor < 0 {
		return 0, 0, false
	}
	return major, minor, true
}

// PageSize returns one page's resolved /MediaBox in points, inheriting from
// the nearest /Pages ancestor and falling back to the reader default of 612 by
// 792 when the tree carries no box. index is zero-based. An index past the
// last page or a malformed box is a JobError with Op Info. A null file is
// typecheck. The PostScript writer uses this so its %%BoundingBox is the
// document's real page and not a fixed letter box.
func (file *File) PageSize(index int) (PageSize, error) {
	if file == nil {
		return PageSize{}, NewError(opInfo, errType)
	}
	sizes, err := file.pageSizes()
	if err != nil {
		return PageSize{}, err
	}
	if index < 0 || index >= len(sizes) {
		return PageSize{}, NewError(opInfo, errRange)
	}
	return sizes[index], nil
}

// pageSizes walks the page tree and resolves /MediaBox with inheritance.
// The nearest ancestor box wins, and a tree with no box reports the reader
// default.
func (file *File) pageSizes() ([]PageSize, error) {
	catalog, err := file.catalogValue()
	if err != nil {
		return nil, err
	}
	pages, ok := catalog.ValueEntry(keyPages)
	if !ok || pages.Kind == KindNull {
		return nil, NewError(opInfo, errSyntax)
	}
	inherited := PageSize{Width: infoDefaultPageWidth, Height: infoDefaultPageHeight}
	sizes := []PageSize{}
	walkErr := file.walkPageSizes(pages, map[int]bool{}, map[int]bool{}, inherited, &sizes)
	if walkErr == nil {
		return sizes, nil
	}
	return file.recoverPageSizes(pages, inherited, walkErr)
}

func (file *File) recoverPageSizes(pages Value, inherited PageSize, walkErr error) ([]PageSize, error) {
	if file.recovered {
		return file.scannedPageSizesOrError(walkErr)
	}
	if !file.pageTreeHasStream(pages) {
		return nil, walkErr
	}
	recovered := []PageSize{}
	file.walkDamagedPageSizes(pages, map[int]bool{}, map[int]bool{}, inherited, &recovered)
	if len(recovered) > 0 {
		return recovered, nil
	}
	return file.scannedPageSizesOrError(walkErr)
}

func (file *File) scannedPageSizesOrError(err error) ([]PageSize, error) {
	if scanned, scanErr := file.scanPageSizes(); scanErr == nil {
		return scanned, nil
	}
	return nil, err
}

// scanPageSizes uses page objects directly when a damaged page tree cannot be
// walked. This follows scanPages order and uses the normal default page box.
func (file *File) scanPageSizes() ([]PageSize, error) {
	nums := make([]int, 0, len(file.xref))
	for num := range file.xref {
		nums = append(nums, num)
	}
	slices.Sort(nums)
	sizes := []PageSize{}
	defaultSize := PageSize{Width: infoDefaultPageWidth, Height: infoDefaultPageHeight}
	for _, num := range nums {
		node, err := file.resolve(num)
		if err != nil || node.Kind != KindDict {
			continue
		}
		typeName, _ := node.NameEntry(keyType)
		if typeName != keyPage {
			continue
		}
		size, err := file.nodeSize(node, defaultSize)
		if err != nil {
			return nil, err
		}
		sizes = append(sizes, size)
	}
	if len(sizes) == 0 {
		return nil, NewError(opInfo, errSyntax)
	}
	return sizes, nil
}

func (file *File) walkDamagedPageSizes(val Value, seen, path map[int]bool, inherited PageSize, sizes *[]PageSize) {
	if val.Kind == KindRef {
		if seen[val.RefNum] || path[val.RefNum] {
			return
		}
		seen[val.RefNum] = true
		path[val.RefNum] = true
		defer delete(path, val.RefNum)
	}
	node, err := file.deref(val)
	if err != nil || node.Kind != KindDict {
		return
	}
	inherited = file.inheritedPageSize(node, inherited)
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		*sizes = append(*sizes, inherited)
		return
	}
	kids, hasKids, err := file.kidArray(node)
	if err != nil || !hasKids {
		return
	}
	file.walkDamagedPageSizeKids(kids, seen, path, inherited, sizes)
}

func (file *File) walkDamagedPageSizeKids(
	kids []Value,
	seen, path map[int]bool,
	inherited PageSize,
	sizes *[]PageSize,
) {
	for _, kid := range kids {
		file.walkDamagedPageSizes(kid, seen, path, inherited, sizes)
	}
}

func (file *File) inheritedPageSize(node Value, inherited PageSize) PageSize {
	size, err := file.nodeSize(node, inherited)
	if err != nil {
		return inherited
	}
	return size
}

// walkPageSizes walks the page tree for /MediaBox. It takes the same two
// visited sets as walkRef: seen skips a node a second /Kids entry names, and
// path still refuses a cycle.
func (file *File) walkPageSizes(val Value, seen, path map[int]bool, inherited PageSize, sizes *[]PageSize) error {
	res, err := file.pageNode(val, seen, path, inherited, sizes)
	if err != nil {
		return err
	}
	if res.done {
		return nil
	}
	kids, hasKids, err := file.kidArray(res.node)
	if err != nil {
		return err
	}
	if !hasKids {
		if res.typeNam != keyPages {
			return NewError(opInfo, errSyntax)
		}
		return nil
	}
	for _, kid := range kids {
		if file.nullNode(kid) {
			continue
		}
		if err := file.walkPageSizes(kid, seen, path, res.size, sizes); err != nil {
			return err
		}
	}
	return nil
}

// pageNodeResult is what one page-tree node resolves to. done is true when
// there is nothing left to walk: the node was a leaf page already appended to
// sizes, or the seen set skipped it as a repeat.
type pageNodeResult struct {
	node    Value
	size    PageSize
	typeNam string
	done    bool
}

// emptyPageNodeResult is the result for a repeated node the seen set skipped
// and for an error that stops the walk. The caller checks done and err before
// it reads node or size, so those never matter. The named zero exists because
// the linter requires every field of a struct to be written out.
func emptyPageNodeResult() pageNodeResult {
	return pageNodeResult{
		node: Value{
			Kind:   KindNull,
			Bool:   false,
			Int:    0,
			Real:   0,
			Name:   "",
			String: "",
			Array:  nil,
			Dict:   nil,
			Stream: nil,
			RefNum: 0,
			RefGen: 0,
		},
		size:    PageSize{Width: 0, Height: 0},
		typeNam: "",
		done:    true,
	}
}

// pageNode resolves one page-tree node: it applies the visited sets and
// returns the node, the size it passes to its children, and its /Type.
func (file *File) pageNode(
	val Value,
	seen, path map[int]bool,
	inherited PageSize,
	sizes *[]PageSize,
) (pageNodeResult, error) {
	if val.Kind == KindRef {
		if path[val.RefNum] {
			empty := emptyPageNodeResult()
			empty.done = false
			return empty, NewError(opInfo, errSyntax)
		}
		if seen[val.RefNum] {
			return emptyPageNodeResult(), nil
		}
		seen[val.RefNum] = true
		path[val.RefNum] = true
		defer delete(path, val.RefNum)
	}
	node, err := file.deref(val)
	if err != nil {
		return emptyPageNodeResult(), err
	}
	if node.Kind != KindDict {
		empty := emptyPageNodeResult()
		empty.done = false
		return empty, NewError(opInfo, errSyntax)
	}
	size, err := file.nodeSize(node, inherited)
	if err != nil {
		return emptyPageNodeResult(), err
	}
	typeName, _ := node.NameEntry(keyType)
	if typeName == keyPage {
		*sizes = append(*sizes, size)
		return pageNodeResult{node: node, size: size, typeNam: typeName, done: true}, nil
	}
	return pageNodeResult{node: node, size: size, typeNam: typeName, done: false}, nil
}

// nodeSize returns the node's own /MediaBox, or the inherited one.
func (file *File) nodeSize(node Value, inherited PageSize) (PageSize, error) {
	entry, ok := node.ValueEntry(keyMediaBox)
	if !ok || entry.Kind == KindNull {
		return inherited, nil
	}
	return file.pageSize(entry)
}

// pageSize resolves one /MediaBox array into points.
func (file *File) pageSize(entry Value) (PageSize, error) {
	box, err := file.deref(entry)
	if err != nil {
		return PageSize{}, err
	}
	if box.Kind != KindArray || len(box.Array) != 4 {
		return PageSize{}, NewError(opInfo, errSyntax)
	}
	var corners [4]float64
	for i, item := range box.Array {
		number, err := file.deref(item)
		if err != nil {
			return PageSize{}, err
		}
		value, ok := numberValueOf(number)
		if !ok {
			return PageSize{}, NewError(opInfo, errSyntax)
		}
		corners[i] = value
	}
	return PageSize{Width: corners[2] - corners[0], Height: corners[3] - corners[1]}, nil
}

func numberValueOf(val Value) (float64, bool) {
	if val.Kind == KindInt {
		return float64(val.Int), true
	}
	if val.Kind == KindReal {
		return val.Real, true
	}
	return 0, false
}

// fontInfos lists every in-use /Type /Font dictionary except CIDFont
// descendants, sorted by name and embedded flag. An in-use row whose object the
// file does not carry is a dead object number, not a font, so the survey steps
// over it; a page that references the same number still fails to resolve.
func (file *File) fontInfos() []FontInfo {
	unique := map[FontInfo]bool{}
	for _, num := range file.inUseNums() {
		val, ok := file.readable(num)
		if !ok {
			continue
		}
		if info, isFont := file.fontInfoValue(val); isFont {
			unique[info] = true
		}
	}
	fonts := make([]FontInfo, 0, len(unique))
	for info := range unique {
		fonts = append(fonts, info)
	}
	slices.SortFunc(fonts, func(a, b FontInfo) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		if a.Embedded == b.Embedded {
			return 0
		}
		if a.Embedded {
			return 1
		}
		return -1
	})
	return fonts
}

// fontInfoValue reads a font row from an already-resolved object. The bool is
// false for any object that is not a top-level font dictionary.
func (file *File) fontInfoValue(val Value) (FontInfo, bool) {
	if val.Kind != KindDict {
		return FontInfo{Name: "", Embedded: false}, false
	}
	if typeName, ok := val.NameEntry(keyType); !ok || typeName != keyFont {
		return FontInfo{Name: "", Embedded: false}, false
	}
	if subtype, ok := val.NameEntry(keySubtype); ok && isCIDFont(subtype) {
		return FontInfo{Name: "", Embedded: false}, false
	}
	return FontInfo{Name: fontName(val), Embedded: file.fontEmbedded(val)}, true
}

func isCIDFont(subtype string) bool {
	return subtype == subtypeCIDFontType0 || subtype == subtypeCIDFontType2
}

// fontName returns the /BaseFont name, or a placeholder when it is missing.
func fontName(val Value) string {
	name, ok := val.NameEntry(keyBaseFont)
	if !ok || name == "" {
		return infoUnknownFont
	}
	return name
}

// fontEmbedded reports whether the font carries an outline program. A Type 3
// font counts as embedded because its glyph procedures live in the file. A
// Type0 font needs every descendant to carry one.
//
//nolint:cyclop // one branch per font subtype.
func (file *File) fontEmbedded(val Value) bool {
	subtype, _ := val.NameEntry(keySubtype)
	if subtype == subtypeType3 {
		return true
	}
	if subtype == subtypeType0 {
		entry, ok := val.ValueEntry(keyDescendantFonts)
		if !ok || entry.Kind == KindNull {
			return false
		}
		array, err := file.deref(entry)
		if err != nil || array.Kind != KindArray || len(array.Array) == 0 {
			return false
		}
		items := array.Array
		for _, item := range items {
			descendant, err := file.deref(item)
			if err != nil || descendant.Kind != KindDict || !file.descriptorEmbedded(descendant) {
				return false
			}
		}
		return true
	}
	return file.descriptorEmbedded(val)
}

// descriptorEmbedded reports whether /FontDescriptor names a /FontFile,
// /FontFile2, or /FontFile3 program.
func (file *File) descriptorEmbedded(val Value) bool {
	entry, ok := val.ValueEntry(keyFontDescriptor)
	if !ok {
		return false
	}
	descriptor, err := file.deref(entry)
	if err != nil || descriptor.Kind != KindDict {
		return false
	}
	for _, key := range []string{keyFontFile, keyFontFile2, keyFontFile3} {
		program, found := descriptor.ValueEntry(key)
		if found && program.Kind != KindNull {
			return true
		}
	}
	return false
}

// inUseNums returns the in-use object numbers in ascending order.
func (file *File) inUseNums() []int {
	nums := make([]int, 0, len(file.xref))
	for num, entry := range file.xref {
		if entry.InUse {
			nums = append(nums, num)
		}
	}
	slices.Sort(nums)
	return nums
}
