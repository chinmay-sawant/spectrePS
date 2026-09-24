package pdf

// Kind is the kind of one PDF value.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindReal
	KindName
	KindString
	KindArray
	KindDict
	KindStream
	KindRef
)

// Value is one PDF object.
// Stream is the raw bytes between stream and endstream. It is still encoded.
// Dict on a stream value is the stream dictionary.
type Value struct {
	Kind   Kind
	Bool   bool
	Int    int64
	Real   float64
	Name   string
	String string
	Array  []Value
	Dict   map[string]Value
	Stream []byte
	RefNum int
	RefGen int
}

// NullVal returns a null object.
func NullVal() Value {
	return Value{
		Kind: KindNull, Bool: false, Int: 0, Real: 0, Name: "", String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// BoolVal returns a boolean object.
func BoolVal(flag bool) Value {
	return Value{
		Kind: KindBool, Bool: flag, Int: 0, Real: 0, Name: "", String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// IntVal returns an integer object.
func IntVal(number int64) Value {
	return Value{
		Kind: KindInt, Bool: false, Int: number, Real: 0, Name: "", String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// RealVal returns a real object.
func RealVal(number float64) Value {
	return Value{
		Kind: KindReal, Bool: false, Int: 0, Real: number, Name: "", String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// NameVal returns a name without the leading slash.
func NameVal(name string) Value {
	return Value{
		Kind: KindName, Bool: false, Int: 0, Real: 0, Name: name, String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// StringVal returns a decoded string.
func StringVal(text string) Value {
	return Value{
		Kind: KindString, Bool: false, Int: 0, Real: 0, Name: "", String: text,
		Array: nil, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// ArrayVal returns an array. items is kept as given.
func ArrayVal(items []Value) Value {
	return Value{
		Kind: KindArray, Bool: false, Int: 0, Real: 0, Name: "", String: "",
		Array: items, Dict: nil, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// DictVal returns a dictionary. Keys have no leading slash.
func DictVal(entries map[string]Value) Value {
	return Value{
		Kind: KindDict, Bool: false, Int: 0, Real: 0, Name: "", String: "",
		Array: nil, Dict: entries, Stream: nil, RefNum: 0, RefGen: 0,
	}
}

// StreamVal returns a stream. raw is still encoded. dict is the stream dictionary.
func StreamVal(dict map[string]Value, raw []byte) Value {
	return Value{
		Kind: KindStream, Bool: false, Int: 0, Real: 0, Name: "", String: "",
		Array: nil, Dict: dict, Stream: raw, RefNum: 0, RefGen: 0,
	}
}

// RefVal returns an indirect reference.
func RefVal(objNum, gen int) Value {
	return Value{
		Kind: KindRef, Bool: false, Int: 0, Real: 0, Name: "", String: "",
		Array: nil, Dict: nil, Stream: nil, RefNum: objNum, RefGen: gen,
	}
}

// ValueEntry returns a dictionary entry.
func (val Value) ValueEntry(key string) (Value, bool) {
	if val.Dict == nil {
		return NullVal(), false
	}
	entry, ok := val.Dict[key]
	if !ok {
		return NullVal(), false
	}
	return entry, true
}

// NameEntry returns a name dictionary entry without a slash.
func (val Value) NameEntry(key string) (string, bool) {
	entry, ok := val.ValueEntry(key)
	if !ok || entry.Kind != KindName {
		return "", false
	}
	return entry.Name, true
}

// IntEntry returns an integer dictionary entry. A whole real is accepted.
func (val Value) IntEntry(key string) (int, bool) {
	entry, ok := val.ValueEntry(key)
	if !ok {
		return 0, false
	}
	if entry.Kind == KindInt {
		return int(entry.Int), true
	}
	if entry.Kind != KindReal {
		return 0, false
	}
	whole := int64(entry.Real)
	if float64(whole) != entry.Real {
		return 0, false
	}
	return int(whole), true
}

// ArrayEntry returns an array dictionary entry.
func (val Value) ArrayEntry(key string) ([]Value, bool) {
	entry, ok := val.ValueEntry(key)
	if !ok || entry.Kind != KindArray {
		return nil, false
	}
	return entry.Array, true
}
