package pdfa

import (
	"context"
	"slices"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	opPDFA    = "PDFA"
	errType   = "typecheck"
	errSyntax = "syntaxerror"
)

// The failed rule names carried in the *pdf.Error Name field.
const (
	ruleFontNotEmbedded      = "font-not-embedded"
	ruleLZWDecode            = "lzwdecode"
	ruleFilterNotAllowed     = "filter-not-allowed"
	ruleCMYKWithoutProfile   = "cmyk-without-profile"
	ruleAlternatesNotAllowed = "alternates-not-allowed"
	ruleOPINotAllowed        = "opi-not-allowed"
	ruleBlendModeNotAllowed  = "blend-mode-not-allowed"
	ruleEmbeddedFilesNeed4F  = "embedded-files-need-4f"
	rule4FNeedsEmbeddedFiles = "4f-needs-embedded-files"
)

const (
	keyType             = "Type"
	keySubtype          = "Subtype"
	keyFontDescriptor   = "FontDescriptor"
	keyDescendants      = "DescendantFonts"
	keyFontFile         = "FontFile"
	keyFontFile2        = "FontFile2"
	keyFontFile3        = "FontFile3"
	keyNames            = "Names"
	keyEmbeddedFiles    = "EmbeddedFiles"
	keyFilter           = "Filter"
	keyColorSpace       = "ColorSpace"
	keyResources        = "Resources"
	keyAlternates       = "Alternates"
	keyAlternate        = "Alternate"
	keyOPI              = "OPI"
	keyBM               = "BM"
	typeFont            = "Font"
	typeType0           = "Type0"
	typeType3           = "Type3"
	nameNormal          = "Normal"
	nameDeviceCMYK      = "DeviceCMYK"
	nameLZWDecode       = "LZWDecode"
	nameCrypt           = "Crypt"
	pdfaNilContextPanic = "pdfa: nil context"
)

// allowedFilter reports whether a filter name is in the ISO 32000-2 filter
// table. LZWDecode and Crypt are handled by their own rules.
func allowedFilter(name string) bool {
	switch name {
	case "ASCIIHexDecode", "ASCII85Decode", "FlateDecode", "RunLengthDecode",
		"CCITTFaxDecode", "JBIG2Decode", "DCTDecode", "JPXDecode":
		return true
	default:
		return false
	}
}

// Preflight returns a *pdf.Error with Op "PDFA" and the failed rule when file
// cannot carry mode. It walks every in-use object, not only the page tree, so
// an unused font or stream can still refuse the claim. ModeNone returns nil.
// A canceled context returns ctx.Err(). A nil context panics with
// "pdfa: nil context".
func Preflight(ctx context.Context, file *pdf.File, mode Mode) error {
	if ctx == nil {
		panic(pdfaNilContextPanic)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if mode == ModeNone {
		return nil
	}
	if mode != Mode4 && mode != Mode4F {
		return pdf.NewError(opPDFA, errType)
	}
	if file == nil {
		return pdf.NewError(opPDFA, errSyntax)
	}
	scan := &scanner{file: file, seen: map[int]bool{}}
	if err := scan.objects(); err != nil {
		return err
	}
	return checkEmbeddedFiles(file, mode)
}

// scanner carries the visited object numbers so a reference cycle terminates.
type scanner struct {
	file *pdf.File
	seen map[int]bool
}

func (scan *scanner) objects() error {
	for num := 1; num <= scan.file.ObjectCount(); num++ {
		val, ok, err := scan.file.ObjectValue(num)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		scan.seen[num] = true
		if err := scan.value(val); err != nil {
			return err
		}
	}
	return nil
}

func (scan *scanner) value(val pdf.Value) error {
	switch val.Kind {
	case pdf.KindRef:
		return scan.ref(val.RefNum)
	case pdf.KindArray:
		for _, item := range val.Array {
			if err := scan.value(item); err != nil {
				return err
			}
		}
	case pdf.KindDict:
		return scan.dict(val)
	case pdf.KindStream:
		if err := scan.filter(val); err != nil {
			return err
		}
		return scan.dict(val)
	case pdf.KindNull, pdf.KindBool, pdf.KindInt, pdf.KindReal, pdf.KindName, pdf.KindString:
		return nil
	}
	return nil
}

func (scan *scanner) ref(num int) error {
	if scan.seen[num] {
		return nil
	}
	scan.seen[num] = true
	val, ok, err := scan.file.ObjectValue(num)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return scan.value(val)
}

func (scan *scanner) dict(val pdf.Value) error {
	if typeName, ok := val.NameEntry(keyType); ok && typeName == typeFont {
		if err := scan.font(val); err != nil {
			return err
		}
	}
	if err := scan.blendMode(val); err != nil {
		return err
	}
	if _, ok := val.ValueEntry(keyAlternates); ok {
		return pdf.NewError(opPDFA, ruleAlternatesNotAllowed)
	}
	if _, ok := val.ValueEntry(keyOPI); ok {
		return pdf.NewError(opPDFA, ruleOPINotAllowed)
	}
	if err := scan.colorSpace(val); err != nil {
		return err
	}
	return scan.resources(val)
}

// resources applies the color space rule to the /ColorSpace subdictionary of a
// resource dictionary. A page, a form, and a pattern each carry one, and the
// dictionary is reached even when it is written directly inside the parent.
func (scan *scanner) resources(val pdf.Value) error {
	entry, ok := val.ValueEntry(keyResources)
	if !ok || entry.Kind == pdf.KindNull {
		return nil
	}
	node, err := scan.resolve(entry)
	if err != nil {
		return err
	}
	if node.Kind != pdf.KindDict {
		return nil
	}
	return scan.colorSpace(node)
}

func (scan *scanner) blendMode(val pdf.Value) error {
	name, ok := val.NameEntry(keyBM)
	if ok && name != nameNormal {
		return pdf.NewError(opPDFA, ruleBlendModeNotAllowed)
	}
	return nil
}

// colorSpace refuses DeviceCMYK on any dictionary entry. The claim appends an
// RGB output intent, so no matching CMYK profile exists. A page /ColorSpace
// resource is a dictionary of names, and a Separation or DeviceN alternate
// names DeviceCMYK inside it.
func (scan *scanner) colorSpace(val pdf.Value) error {
	entry, ok := val.ValueEntry(keyColorSpace)
	if !ok {
		return nil
	}
	found, err := scan.deviceCMYK(entry, map[int]bool{})
	if err != nil {
		return err
	}
	if found {
		return pdf.NewError(opPDFA, ruleCMYKWithoutProfile)
	}
	return nil
}

func (scan *scanner) deviceCMYK(entry pdf.Value, visited map[int]bool) (bool, error) {
	switch entry.Kind {
	case pdf.KindRef:
		if visited[entry.RefNum] {
			return false, nil
		}
		visited[entry.RefNum] = true
		resolved, err := scan.resolve(entry)
		if err != nil {
			return false, err
		}
		return scan.deviceCMYK(resolved, visited)
	case pdf.KindName:
		return entry.Name == nameDeviceCMYK, nil
	case pdf.KindArray:
		return scan.deviceCMYKArray(entry, visited)
	case pdf.KindDict:
		return scan.deviceCMYKDict(entry, visited)
	case pdf.KindStream:
		return scan.deviceCMYKStream(entry, visited)
	case pdf.KindNull, pdf.KindBool, pdf.KindInt, pdf.KindReal, pdf.KindString:
		return false, nil
	}
	return false, nil
}

// deviceCMYKArray walks one color space array. A Separation, DeviceN, or
// Indexed value names its spaces in the array elements.
func (scan *scanner) deviceCMYKArray(entry pdf.Value, visited map[int]bool) (bool, error) {
	for _, item := range entry.Array {
		found, err := scan.deviceCMYK(item, visited)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// deviceCMYKDict walks a color space resource dictionary: each value names one
// space, and a Separation or DeviceN alternate is reached through it.
func (scan *scanner) deviceCMYKDict(entry pdf.Value, visited map[int]bool) (bool, error) {
	keys := make([]string, 0, len(entry.Dict))
	for key := range entry.Dict {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		found, err := scan.deviceCMYK(entry.Dict[key], visited)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// deviceCMYKStream walks an ICCBased profile stream. Its /Alternate names the
// space the samples preview as, and DeviceCMYK there has no matching profile.
func (scan *scanner) deviceCMYKStream(entry pdf.Value, visited map[int]bool) (bool, error) {
	alternate, ok := entry.ValueEntry(keyAlternate)
	if !ok || alternate.Kind == pdf.KindNull {
		return false, nil
	}
	return scan.deviceCMYK(alternate, visited)
}

func (scan *scanner) font(val pdf.Value) error {
	if subtype, ok := val.NameEntry(keySubtype); ok {
		switch subtype {
		case typeType3:
			return nil
		case typeType0:
			return scan.type0Font(val)
		}
	}
	if scan.embeddedFont(val) {
		return nil
	}
	return pdf.NewError(opPDFA, ruleFontNotEmbedded)
}

// type0Font requires every descendant CIDFont to carry an embedded file.
func (scan *scanner) type0Font(val pdf.Value) error {
	items, ok := val.ArrayEntry(keyDescendants)
	if !ok || len(items) == 0 {
		return pdf.NewError(opPDFA, ruleFontNotEmbedded)
	}
	for _, item := range items {
		descendant, err := scan.resolve(item)
		if err != nil {
			return err
		}
		if descendant.Kind != pdf.KindDict || !scan.embeddedFont(descendant) {
			return pdf.NewError(opPDFA, ruleFontNotEmbedded)
		}
	}
	return nil
}

// embeddedFont reports whether the font dictionary names a descriptor with a
// FontFile, FontFile2, or FontFile3 entry.
func (scan *scanner) embeddedFont(val pdf.Value) bool {
	entry, ok := val.ValueEntry(keyFontDescriptor)
	if !ok {
		return false
	}
	descriptor, err := scan.resolve(entry)
	if err != nil || descriptor.Kind != pdf.KindDict {
		return false
	}
	for _, key := range []string{keyFontFile, keyFontFile2, keyFontFile3} {
		if file, found := descriptor.ValueEntry(key); found && file.Kind != pdf.KindNull {
			return true
		}
	}
	return false
}

// filter refuses LZWDecode and any filter name outside the standard table.
func (scan *scanner) filter(val pdf.Value) error {
	entry, ok := val.ValueEntry(keyFilter)
	if !ok || entry.Kind == pdf.KindNull {
		return nil
	}
	names, ok := filterNames(entry)
	if !ok {
		return pdf.NewError(opPDFA, ruleFilterNotAllowed)
	}
	for _, name := range names {
		if name == nameLZWDecode {
			return pdf.NewError(opPDFA, ruleLZWDecode)
		}
		if name == nameCrypt || !allowedFilter(name) {
			return pdf.NewError(opPDFA, ruleFilterNotAllowed)
		}
	}
	return nil
}

func filterNames(entry pdf.Value) ([]string, bool) {
	if entry.Kind == pdf.KindName {
		return []string{entry.Name}, true
	}
	if entry.Kind != pdf.KindArray {
		return nil, false
	}
	names := make([]string, 0, len(entry.Array))
	for _, item := range entry.Array {
		if item.Kind != pdf.KindName {
			return nil, false
		}
		names = append(names, item.Name)
	}
	return names, true
}

// resolve dereferences one indirect object without marking it visited.
func (scan *scanner) resolve(val pdf.Value) (pdf.Value, error) {
	if val.Kind != pdf.KindRef {
		return val, nil
	}
	got, ok, err := scan.file.ObjectValue(val.RefNum)
	if err != nil {
		return pdf.NullVal(), err
	}
	if !ok {
		return pdf.NullVal(), nil
	}
	return got, nil
}

// checkEmbeddedFiles compares the catalog /Names /EmbeddedFiles entry with the
// mode: PDF/A-4 base refuses embedded files, and 4f requires them.
func checkEmbeddedFiles(file *pdf.File, mode Mode) error {
	has, err := hasEmbeddedFiles(file)
	if err != nil {
		return err
	}
	switch {
	case mode == Mode4 && has:
		return pdf.NewError(opPDFA, ruleEmbeddedFilesNeed4F)
	case mode == Mode4F && !has:
		return pdf.NewError(opPDFA, rule4FNeedsEmbeddedFiles)
	}
	return nil
}

func hasEmbeddedFiles(file *pdf.File) (bool, error) {
	names, err := catalogNames(file)
	if err != nil || names.Kind != pdf.KindDict {
		return false, err
	}
	entry, ok := names.ValueEntry(keyEmbeddedFiles)
	return ok && entry.Kind != pdf.KindNull, nil
}

// catalogNames returns the resolved catalog /Names value, or a null value.
func catalogNames(file *pdf.File) (pdf.Value, error) {
	root := file.RootNum()
	if root <= 0 {
		return pdf.NullVal(), nil
	}
	catalog, ok, err := file.ObjectValue(root)
	if err != nil || !ok || catalog.Kind != pdf.KindDict {
		return pdf.NullVal(), err
	}
	names, ok := catalog.ValueEntry(keyNames)
	if !ok {
		return pdf.NullVal(), nil
	}
	if names.Kind != pdf.KindRef {
		return names, nil
	}
	resolved, ok, err := file.ObjectValue(names.RefNum)
	if err != nil || !ok {
		return pdf.NullVal(), err
	}
	return resolved, nil
}
