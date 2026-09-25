package pdf

// PaintOptions carries the resolved page resources for one content stream.
type PaintOptions struct {
	// Resources holds the page resources the interpreter reads.
	Resources Resources
	// Text carries the font override and the glyph sink.
	Text TextOptions
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
}

// PageResources returns the effective resources of a zero-based page.
// The nearest /Resources on the page or a /Pages ancestor wins, and the
// /XObject and /Font subdictionaries resolve through references.
// A bad index is rangecheck.
func (file *File) PageResources(index int) (Resources, error) {
	if file == nil || index < 0 || index >= len(file.resources) {
		return Resources{XObjects: nil, Fonts: nil}, NewError(opRaster, errRange)
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
	out := Resources{XObjects: map[string]Value{}, Fonts: map[string]*Font{}}
	if val.Kind == KindNull {
		return out, nil
	}
	node, err := file.deref(val)
	if err != nil {
		return Resources{XObjects: nil, Fonts: nil}, err
	}
	if node.Kind != KindDict {
		return out, nil
	}
	if err := file.resolveXObjects(node, &out); err != nil {
		return Resources{XObjects: nil, Fonts: nil}, err
	}
	if err := file.resolveFonts(node, &out); err != nil {
		return Resources{XObjects: nil, Fonts: nil}, err
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
