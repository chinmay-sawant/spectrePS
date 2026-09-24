package ps

// newDict returns an empty dictionary. system marks systemdict.
func newDict(system bool) *Dict {
	return &Dict{
		system: system,
		keys:   nil,
		vals:   make(map[string]Object),
	}
}

func (d *Dict) get(name string) (Object, bool) {
	if d == nil || d.vals == nil {
		return NullObj(), false
	}
	obj, ok := d.vals[name]
	return obj, ok
}

func (d *Dict) has(name string) bool {
	_, ok := d.get(name)
	return ok
}

// putNew inserts name or replaces its value. A replacement keeps the old key position.
func (d *Dict) putNew(name string, val Object) {
	if _, exists := d.vals[name]; !exists {
		d.keys = append(d.keys, name)
	}
	d.vals[name] = val
}

// Len returns the number of entries.
func (d *Dict) Len() int {
	if d == nil {
		return 0
	}
	return len(d.keys)
}

// Keys returns a copy of the keys in insertion order.
func (d *Dict) Keys() []string {
	if d == nil {
		return []string{}
	}
	out := make([]string, len(d.keys))
	copy(out, d.keys)
	return out
}

func (d *Dict) systemDict() bool {
	return d != nil && d.system
}
