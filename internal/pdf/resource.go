package pdf

const (
	keyExtGState  = "ExtGState"
	keyProperties = "Properties"
)

// MarkedContentSink receives marked-content events in stream order.
// BeginMarkedContent carries the tag, the resolved /Properties value, and the
// nesting depth, which is 1 for the outermost sequence. EndMarkedContent
// receives the depth of the matching Begin at EMC. MP and DP fire no event
// because they have no matching EMC. A nil sink keeps the old behavior.
type MarkedContentSink interface {
	BeginMarkedContent(tag string, properties Value, depth int)
	EndMarkedContent(depth int)
}

// PaintOptions carries the resolved page resources for one content stream.
type PaintOptions struct {
	// Resources holds the page resources the interpreter reads.
	Resources Resources
	// Text carries the font override and the glyph sink.
	Text TextOptions
	// MarkedContent receives BMC, BDC, and EMC events. A nil sink discards them.
	MarkedContent MarkedContentSink
}

// Resources is the page resource subset the content interpreter reads.
// XObjects maps each /XObject name to its resolved value.
// Fonts maps each /Font name to its loaded font.
type Resources struct {
	// XObjects holds the /XObject subdictionary by name. A value is resolved
	// through its indirect reference, so the image stream is ready to decode.
	XObjects map[string]Value
	// Fonts holds the /Font subdictionary by name, already parsed.
	Fonts map[string]*Font
	// ExtGStates holds the /ExtGState subdictionary by name, resolved.
	ExtGStates map[string]Value
	// Properties holds the /Properties subdictionary by name. An entry keeps
	// its indirect reference so the optional content check can compare the
	// object number against /OCProperties /D /OFF; the consumer resolves it.
	Properties map[string]Value
	// Colors holds the /ColorSpace subdictionary by name, resolved.
	Colors map[string]Value
	// file resolves the resources of a nested form XObject. It is nil for a
	// Resources value built without a file, and a form that names its own
	// resources then refuses with undefined in Do.
	file *File
}

// emptyResources returns a Resources value with no entries.
func emptyResources() Resources {
	return Resources{
		XObjects: nil, Fonts: nil, ExtGStates: nil, Properties: nil,
		Colors: nil, file: nil,
	}
}

// ExtGState returns the resolved /ExtGState entry for a resource name.
func (res Resources) ExtGState(name string) (Value, bool) {
	entry, ok := res.ExtGStates[name]
	return entry, ok
}

// Property returns the resolved /Properties entry for a resource name.
func (res Resources) Property(name string) (Value, bool) {
	entry, ok := res.Properties[name]
	return entry, ok
}

// PageResources returns the effective resources of a zero-based page.
// The nearest /Resources on the page or a /Pages ancestor wins, and the
// /XObject and /Font subdictionaries resolve through references.
// A bad index is rangecheck.
func (file *File) PageResources(index int) (Resources, error) {
	if file == nil || index < 0 || index >= len(file.resources) {
		return emptyResources(), NewError(opRaster, errRange)
	}
	return file.resolveResources(file.resources[index])
}

// Font returns the font for a resource name.
func (res Resources) Font(name string) (*Font, bool) {
	fnt, ok := res.Fonts[name]
	return fnt, ok && fnt != nil
}

// PageFont returns the font for a resource name on a zero-based page. Its
// error is the PageResources error, so a bad index is rangecheck and a
// missing name is undefined.
func (file *File) PageFont(index int, name string) (*Font, error) {
	res, err := file.PageResources(index)
	if err != nil {
		return nil, err
	}
	fnt, ok := res.Font(name)
	if !ok {
		return nil, NewError(opTextFont, errUndefined)
	}
	return fnt, nil
}

// resolveResources reads one /Resources value into the interpreter subset.
// A missing, null, or non-dictionary value yields no resources.
func (file *File) resolveResources(val Value) (Resources, error) {
	out := Resources{
		XObjects:   map[string]Value{},
		Fonts:      map[string]*Font{},
		ExtGStates: map[string]Value{},
		Properties: map[string]Value{},
		Colors:     map[string]Value{},
		file:       file,
	}
	if val.Kind == KindNull {
		return out, nil
	}
	node, err := file.deref(val)
	if err != nil {
		return emptyResources(), err
	}
	if node.Kind != KindDict {
		return out, nil
	}
	if err := file.resolveXObjects(node, &out); err != nil {
		return emptyResources(), err
	}
	if err := file.resolveFonts(node, &out); err != nil {
		return emptyResources(), err
	}
	if err := file.resolveExtGStates(node, &out); err != nil {
		return emptyResources(), err
	}
	if err := file.resolveProperties(node, &out); err != nil {
		return emptyResources(), err
	}
	if err := file.resolveColors(node, &out); err != nil {
		return emptyResources(), err
	}
	return out, nil
}

// resolveExtGStates loads every /ExtGState entry as a resolved value.
func (file *File) resolveExtGStates(node Value, out *Resources) error {
	sub, err := file.resolveSubdict(node, keyExtGState)
	if err != nil {
		return err
	}
	out.ExtGStates = sub
	return nil
}

// resolveProperties loads every /Properties entry and keeps its reference, so
// the optional content check can compare the object number against the OFF
// list. The consumer resolves the entry before it reads it.
func (file *File) resolveProperties(node Value, out *Resources) error {
	sub, err := file.resolveRawSubdict(node, keyProperties)
	if err != nil {
		return err
	}
	out.Properties = sub
	return nil
}

// resolveColors loads every /ColorSpace entry as a resolved value. A nested
// reference inside a color space array stays indirect for the resolver.
func (file *File) resolveColors(node Value, out *Resources) error {
	sub, err := file.resolveSubdict(node, keyColorSpace)
	if err != nil {
		return err
	}
	out.Colors = sub
	return nil
}

// resolveSubdict reads one named subdictionary of /Resources and resolves each
// entry through its indirect reference. A missing or null entry is empty.
func (file *File) resolveSubdict(node Value, key string) (map[string]Value, error) {
	out := map[string]Value{}
	entry, ok := node.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return out, nil
	}
	sub, err := file.deref(entry)
	if err != nil {
		return nil, err
	}
	if sub.Kind != KindDict {
		return out, nil
	}
	for name, item := range sub.Dict {
		resolved, err := file.deref(item)
		if err != nil {
			return nil, err
		}
		out[name] = resolved
	}
	return out, nil
}

// resolveRawSubdict reads one named subdictionary of /Resources and keeps each
// entry as written, so an indirect reference survives. The caller resolves it.
func (file *File) resolveRawSubdict(node Value, key string) (map[string]Value, error) {
	out := map[string]Value{}
	entry, ok := node.ValueEntry(key)
	if !ok || entry.Kind == KindNull {
		return out, nil
	}
	sub, err := file.deref(entry)
	if err != nil {
		return nil, err
	}
	if sub.Kind != KindDict {
		return out, nil
	}
	for name, item := range sub.Dict {
		out[name] = item
	}
	return out, nil
}

func (file *File) resolveXObjects(node Value, out *Resources) error {
	entry, ok := node.ValueEntry(keyXObject)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	sub, err := file.deref(entry)
	if err != nil {
		return err
	}
	if sub.Kind != KindDict {
		return nil
	}
	for name, item := range sub.Dict {
		resolved, err := file.deref(item)
		if err != nil {
			return err
		}
		out.XObjects[name] = resolved
	}
	return nil
}

// resolveFonts loads every /Font entry. Unsupported programs load without an
// outline source and paint as invalidfont; a malformed dictionary fails the
// page.
func (file *File) resolveFonts(node Value, out *Resources) error {
	entry, ok := node.ValueEntry(keyFont)
	if !ok || entry.Kind == KindNull {
		return nil
	}
	sub, err := file.deref(entry)
	if err != nil {
		return err
	}
	if sub.Kind != KindDict {
		return nil
	}
	for name, item := range sub.Dict {
		resolved, err := file.deref(item)
		if err != nil {
			return err
		}
		loaded, err := file.loadFont(resolved)
		if err != nil {
			return err
		}
		out.Fonts[name] = loaded
	}
	return nil
}
