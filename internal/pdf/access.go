package pdf

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ObjectCount returns the highest in-use object number, or 0 when none is in use.
func (file *File) ObjectCount() int {
	if file == nil {
		return 0
	}
	highest := 0
	for num, entry := range file.xref {
		if entry.InUse && num > highest {
			highest = num
		}
	}
	return highest
}

// RootNum returns the object number of the trailer /Root reference, or 0 when
// the trailer has no reference there.
func (file *File) RootNum() int {
	if file == nil {
		return 0
	}
	root, ok := file.trailer.ValueEntry(keyRoot)
	if !ok || root.Kind != KindRef {
		return 0
	}
	return root.RefNum
}

// RawObject returns the stored body bytes of one uncompressed object, without
// the "num gen obj" header and the trailing endobj.
// An object that lives in an object stream, a free or missing number, and a
// number the reader cannot parse all return false.
func (file *File) RawObject(num int) ([]byte, bool) {
	if file == nil {
		return nil, false
	}
	entry, ok := file.xref[num]
	if !ok || !entry.InUse || entry.Compressed {
		return nil, false
	}
	got, gen, _, next, err := parseIndirect(file.src, entry.Offset, file)
	if err != nil || got != num || gen != entry.Gen {
		return nil, false
	}
	start, ok := valueStart(file.src, entry.Offset)
	if !ok {
		return nil, false
	}
	end := trimSpaceEnd(file.src, start, next-len(wordEndObj))
	if end < start {
		return nil, false
	}
	return bytes.Clone(file.src[start:end]), true
}

// trimSpaceEnd moves end back over PDF whitespace, but not past start.
func trimSpaceEnd(src []byte, start, end int) int {
	for end > start && isSpaceByte(src[end-1]) {
		end--
	}
	return end
}

// valueStart returns the first byte of the object value, after "num gen obj".
func valueStart(src []byte, offset int) (int, bool) {
	_, pos, ok := pdfInt(src, offset)
	if !ok {
		return 0, false
	}
	_, pos, ok = pdfInt(src, pos)
	if !ok {
		return 0, false
	}
	pos = skipSpace(src, pos)
	if !hasKeyword(src, pos, wordObj) {
		return 0, false
	}
	return skipSpace(src, pos+len(wordObj)), true
}

// ObjectValue resolves any in-use object. The bool is false for a free or
// missing number. A number that is in use but does not resolve returns true
// with the error.
func (file *File) ObjectValue(num int) (Value, bool, error) {
	if file == nil {
		return NullVal(), false, nil
	}
	entry, ok := file.xref[num]
	if !ok || !entry.InUse {
		return NullVal(), false, nil
	}
	val, err := file.resolve(num)
	if err != nil {
		return NullVal(), true, err
	}
	return val, true, nil
}

// SerializeValue writes one value in PDF syntax.
// Dictionary keys are sorted, so two calls on the same value return equal
// bytes. A stream writes its raw bytes and sets /Length to that byte count.
func SerializeValue(val Value) []byte {
	var buf bytes.Buffer
	writeValue(&buf, val)
	return buf.Bytes()
}

func writeValue(buf *bytes.Buffer, val Value) {
	switch val.Kind {
	case KindArray:
		writeArray(buf, val.Array)
	case KindDict:
		writeDict(buf, val.Dict)
	case KindStream:
		writeStream(buf, val)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindString, KindRef:
		writeScalar(buf, val)
	}
}

func writeScalar(buf *bytes.Buffer, val Value) {
	switch val.Kind {
	case KindNull:
		buf.WriteString(wordNull)
	case KindBool:
		buf.WriteString(strconv.FormatBool(val.Bool))
	case KindInt:
		buf.WriteString(strconv.FormatInt(val.Int, decBase))
	case KindReal:
		writeReal(buf, val.Real)
	case KindName:
		writeName(buf, val.Name)
	case KindString:
		writeString(buf, val.String)
	case KindRef:
		fmt.Fprintf(buf, "%d %d %s", val.RefNum, val.RefGen, wordRef)
	case KindArray, KindDict, KindStream:
		writeValue(buf, val)
	}
}

// writeReal keeps a decimal point so the value stays a real. A non-finite
// number cannot be written and becomes 0.0.
func writeReal(buf *bytes.Buffer, number float64) {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		buf.WriteString("0.0")
		return
	}
	text := strconv.FormatFloat(number, 'f', -1, bitSize)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	buf.WriteString(text)
}

func writeName(buf *bytes.Buffer, name string) {
	buf.WriteByte('/')
	for i := range len(name) {
		cur := name[i]
		if nameByteSafe(cur) {
			buf.WriteByte(cur)
			continue
		}
		fmt.Fprintf(buf, "#%02X", cur)
	}
}

// nameByteSafe reports whether one byte needs no # escape in a name.
func nameByteSafe(cur byte) bool {
	if cur <= ' ' || cur > '~' {
		return false
	}
	switch cur {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%', '#':
		return false
	default:
		return true
	}
}

func writeString(buf *bytes.Buffer, text string) {
	buf.WriteByte('(')
	for i := range len(text) {
		writeStringByte(buf, text[i])
	}
	buf.WriteByte(')')
}

// writeStringByte escapes a byte so the literal string reads back unchanged.
// A raw carriage return would normalize to a line feed, so it is escaped.
func writeStringByte(buf *bytes.Buffer, cur byte) {
	switch cur {
	case '\\', '(', ')':
		buf.WriteByte('\\')
		buf.WriteByte(cur)
	case '\n':
		buf.WriteString(`\n`)
	case '\r':
		buf.WriteString(`\r`)
	case '\t':
		buf.WriteString(`\t`)
	case '\b':
		buf.WriteString(`\b`)
	case '\f':
		buf.WriteString(`\f`)
	default:
		if cur < ' ' || cur > '~' {
			fmt.Fprintf(buf, `\%03o`, cur)
			return
		}
		buf.WriteByte(cur)
	}
}

func writeArray(buf *bytes.Buffer, items []Value) {
	buf.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			buf.WriteByte(' ')
		}
		writeValue(buf, item)
	}
	buf.WriteByte(']')
}

func writeDict(buf *bytes.Buffer, entries map[string]Value) {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buf.WriteString("<<")
	for _, key := range keys {
		buf.WriteByte(' ')
		writeName(buf, key)
		buf.WriteByte(' ')
		writeValue(buf, entries[key])
	}
	if len(keys) > 0 {
		buf.WriteByte(' ')
	}
	buf.WriteString(">>")
}

func writeStream(buf *bytes.Buffer, val Value) {
	writeDict(buf, streamEntries(val))
	buf.WriteString("\nstream\n")
	buf.Write(val.Stream)
	buf.WriteString("\nendstream")
}

// streamEntries copies the stream dictionary with /Length set to the raw byte
// count, so the written stream reads back with its own bytes.
func streamEntries(val Value) map[string]Value {
	entries := make(map[string]Value, len(val.Dict)+1)
	for key, entry := range val.Dict {
		entries[key] = entry
	}
	entries[wordLength] = IntVal(int64(len(val.Stream)))
	return entries
}
