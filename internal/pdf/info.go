package pdf

import (
	"slices"
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
	sizes, err := file.pageSizes()
	if err != nil {
		return InfoReport{}, err
	}
	fonts, err := file.fontInfos()
	if err != nil {
		return InfoReport{}, err
	}
	images, err := file.ImageObjectNums()
	if err != nil {
		return InfoReport{}, err
	}
	return InfoReport{
		Version:   file.version(),
		Pages:     len(sizes),
		PageSizes: sizes,
		Tagged:    file.HasStructTree(),
		Fonts:     fonts,
		Images:    len(images),
	}, nil
}

// version returns the header version, for example "1.4". A file without a
// header returns an empty string.
func (file *File) version() string {
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
	if err := file.walkPageSizes(pages, map[int]bool{}, inherited, &sizes); err != nil {
		return nil, err
	}
	return sizes, nil
}

func (file *File) walkPageSizes(val Value, seen map[int]bool, inherited PageSize, sizes *[]PageSize) error {
	if val.Kind == KindRef {
		if seen[val.RefNum] {
			return NewError(opInfo, errSyntax)
		}
		seen[val.RefNum] = true
	}
	node, err := file.deref(val)
	if err != nil {
		return err
	}
	if node.Kind != KindDict {
		return NewError(opInfo, errSyntax)
	}
	size, err := file.nodeSize(node, inherited)
	if err != nil {
		return err
	}
	if typeName, _ := node.NameEntry(keyType); typeName == keyPage {
		*sizes = append(*sizes, size)
		return nil
	}
	kids, hasKids := node.ArrayEntry(keyKids)
	if !hasKids {
		return NewError(opInfo, errSyntax)
	}
	for _, kid := range kids {
		if err := file.walkPageSizes(kid, seen, size, sizes); err != nil {
			return err
		}
	}
	return nil
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
// descendants, sorted by name and embedded flag.
func (file *File) fontInfos() ([]FontInfo, error) {
	unique := map[FontInfo]bool{}
	for _, num := range file.inUseNums() {
		info, ok, err := file.fontInfo(num)
		if err != nil {
			return nil, err
		}
		if ok {
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
	return fonts, nil
}

// fontInfo resolves one object into a font row. The bool is false for any
// object that is not a top-level font dictionary.
func (file *File) fontInfo(num int) (FontInfo, bool, error) {
	val, ok, err := file.ObjectValue(num)
	if err != nil {
		return FontInfo{Name: "", Embedded: false}, false, err
	}
	if !ok || val.Kind != KindDict {
		return FontInfo{Name: "", Embedded: false}, false, nil
	}
	if typeName, ok := val.NameEntry(keyType); !ok || typeName != keyFont {
		return FontInfo{Name: "", Embedded: false}, false, nil
	}
	if subtype, ok := val.NameEntry(keySubtype); ok && isCIDFont(subtype) {
		return FontInfo{Name: "", Embedded: false}, false, nil
	}
	return FontInfo{Name: fontName(val), Embedded: file.fontEmbedded(val)}, true, nil
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
