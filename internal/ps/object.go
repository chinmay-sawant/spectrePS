// Package ps executes the PostScript subset.
package ps

import "context"

// Kind identifies an object variant.
type Kind int

const (
	// KindNull is the null object.
	KindNull Kind = iota
	// KindBool is a boolean.
	KindBool
	// KindInt is an int32.
	KindInt
	// KindReal is a float64.
	KindReal
	// KindName is a name. Exec is false for a literal name and true for an executable name.
	KindName
	// KindMark is a mark.
	KindMark
	// KindString is a string.
	KindString
	// KindArray is an array or a procedure.
	KindArray
	// KindDict is a dictionary.
	KindDict
	// KindOp is an operator.
	KindOp
)

// Str is a PostScript string. Copies of the object share Bytes.
type Str struct {
	Bytes []byte
	Exec  bool
}

// Arr is a PostScript array or procedure. Copies of the object share Elems.
type Arr struct {
	Elems []Object
	Exec  bool
}

// Dict is a PostScript dictionary. Keys stay in insertion order.
type Dict struct {
	system bool
	keys   []string
	vals   map[string]Object
}

// Operator is one built-in operator.
type Operator func(ctx context.Context, ip *Interp) error

// Object is one PostScript value.
// Simple values are copied with the struct. String, array, and dictionary values share their pointer.
type Object struct {
	Kind Kind
	Exec bool
	Int  int32
	Real float64
	Bool bool
	Name string
	Str  *Str
	Arr  *Arr
	Dict *Dict
	Op   Operator
}

// NullObj returns null.
func NullObj() Object {
	return newObject(KindNull, false, 0, 0, false, "", nil, nil, nil, nil)
}

// BoolObj returns a boolean.
func BoolObj(flag bool) Object {
	return newObject(KindBool, false, 0, 0, flag, "", nil, nil, nil, nil)
}

// IntObj returns an integer.
func IntObj(num int32) Object {
	return newObject(KindInt, false, num, 0, false, "", nil, nil, nil, nil)
}

// RealObj returns a real.
func RealObj(num float64) Object {
	return newObject(KindReal, false, 0, num, false, "", nil, nil, nil, nil)
}

// MarkObj returns a mark.
func MarkObj() Object {
	return newObject(KindMark, false, 0, 0, false, "", nil, nil, nil, nil)
}

// LiteralName returns a literal name. Exec is false.
func LiteralName(name string) Object {
	return newObject(KindName, false, 0, 0, false, name, nil, nil, nil, nil)
}

// ExecName returns an executable name. Exec is true.
func ExecName(name string) Object {
	return newObject(KindName, true, 0, 0, false, name, nil, nil, nil, nil)
}

// StringObj returns a string object. The bytes are copied. exec is the executable bit.
func StringObj(raw []byte, exec bool) Object {
	dup := make([]byte, len(raw))
	copy(dup, raw)
	str := &Str{Bytes: dup, Exec: exec}
	return newObject(KindString, exec, 0, 0, false, "", str, nil, nil, nil)
}

// ArrayObj returns an array object.
// elems are copied into a new slice. Nested composites are still shared.
// exec sets both Arr.Exec and Object.Exec.
func ArrayObj(elems []Object, exec bool) Object {
	dup := make([]Object, len(elems))
	copy(dup, elems)
	arr := &Arr{Elems: dup, Exec: exec}
	return newObject(KindArray, exec, 0, 0, false, "", nil, arr, nil, nil)
}

// DictObj returns a dictionary object. The dictionary is shared.
func DictObj(dict *Dict) Object {
	return newObject(KindDict, false, 0, 0, false, "", nil, nil, dict, nil)
}

// OpObj returns an operator object. Operators are executable.
func OpObj(fn Operator) Object {
	return newObject(KindOp, true, 0, 0, false, "", nil, nil, nil, fn)
}

func newObject(
	kind Kind,
	exec bool,
	num int32,
	numReal float64,
	flag bool,
	name string,
	str *Str,
	arr *Arr,
	dict *Dict,
	fn Operator,
) Object {
	return Object{
		Kind: kind,
		Exec: exec,
		Int:  num,
		Real: numReal,
		Bool: flag,
		Name: name,
		Str:  str,
		Arr:  arr,
		Dict: dict,
		Op:   fn,
	}
}
