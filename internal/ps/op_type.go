package ps

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
)

func registerTypeOps(interp *Interp) {
	interp.Install("type", opType)
	interp.Install("xcheck", opXCheck)
	interp.Install("cvi", opCvi)
	interp.Install("cvr", opCvr)
	interp.Install("cvs", opCvs)
	interp.Install("cvn", opCvn)
	interp.Install("cvx", opCvx)
	interp.Install("cvlit", opCvlit)
	interp.Install("length", opLength)
	interp.Install("get", opGet)
	interp.Install("put", opPut)
	interp.Install("null", opNull)
}

func opType(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return interp.Push(LiteralName(typeName(obj.Kind)))
}

func typeName(kind Kind) string {
	names := map[Kind]string{
		KindNull:   "nulltype",
		KindBool:   "booleantype",
		KindInt:    "integertype",
		KindReal:   "realtype",
		KindName:   "nametype",
		KindMark:   "marktype",
		KindString: "stringtype",
		KindArray:  "arraytype",
		KindDict:   "dicttype",
		KindOp:     "operatortype",
	}
	name, ok := names[kind]
	if !ok {
		return "nulltype"
	}
	return name
}

func opXCheck(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return interp.Push(BoolObj(objExecutable(obj)))
}

func objExecutable(obj Object) bool {
	switch obj.Kind {
	case KindName:
		return obj.Exec
	case KindArray:
		return obj.Exec || arrExecutable(obj.Arr)
	case KindString:
		return obj.Exec || strExecutable(obj.Str)
	case KindOp:
		return true
	case KindNull, KindBool, KindInt, KindReal, KindMark, KindDict:
		return false
	default:
		return false
	}
}

func arrExecutable(arr *Arr) bool {
	return arr != nil && arr.Exec
}

func strExecutable(str *Str) bool {
	return str != nil && str.Exec
}

func opCvi(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	value, err := cviValue(obj)
	if err != nil {
		return err
	}
	return interp.Push(IntObj(value))
}

func cviValue(obj Object) (int32, error) {
	switch obj.Kind {
	case KindInt:
		return obj.Int, nil
	case KindReal:
		return truncInt(obj.Real, "cvi")
	case KindString:
		return cviString(obj)
	case KindNull, KindBool, KindName, KindMark, KindArray, KindDict, KindOp:
		return 0, errOf(errTypecheck, "cvi")
	default:
		return 0, errOf(errTypecheck, "cvi")
	}
}

func cviString(obj Object) (int32, error) {
	if obj.Str == nil {
		return 0, errOf(errSyntax, "cvi")
	}
	value, err := parsePSNumber(obj.Str.Bytes, "cvi")
	if err != nil {
		return 0, err
	}
	return truncInt(value, "cvi")
}

func truncInt(value float64, opName string) (int32, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errOf(errRangecheck, opName)
	}
	trunc := math.Trunc(value)
	if trunc > math.MaxInt32 || trunc < math.MinInt32 {
		return 0, errOf(errRangecheck, opName)
	}
	return int32(trunc), nil
}

func opCvr(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	value, err := cvrValue(obj)
	if err != nil {
		return err
	}
	return interp.Push(RealObj(value))
}

func cvrValue(obj Object) (float64, error) {
	switch obj.Kind {
	case KindInt:
		return float64(obj.Int), nil
	case KindReal:
		return obj.Real, nil
	case KindString:
		return cvrString(obj)
	case KindNull, KindBool, KindName, KindMark, KindArray, KindDict, KindOp:
		return 0, errOf(errTypecheck, "cvr")
	default:
		return 0, errOf(errTypecheck, "cvr")
	}
}

func cvrString(obj Object) (float64, error) {
	if obj.Str == nil {
		return 0, errOf(errSyntax, "cvr")
	}
	value, err := parsePSNumber(obj.Str.Bytes, "cvr")
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parsePSNumber(raw []byte, opName string) (float64, error) {
	text := trimPSSpace(raw)
	isReal, err := numberShape(text, opName)
	if err != nil {
		return 0, err
	}
	if isReal {
		return parseRealToken(text, opName)
	}
	return parseIntegerToken(text, opName)
}

func trimPSSpace(raw []byte) []byte {
	start := 0
	for start < len(raw) && isPSSpace(raw[start]) {
		start++
	}
	end := len(raw)
	for end > start && isPSSpace(raw[end-1]) {
		end--
	}
	return raw[start:end]
}

func isPSSpace(b byte) bool {
	return strings.ContainsRune(psWhitespace, rune(b))
}

func numberShape(text []byte, opName string) (bool, error) {
	if len(text) == 0 {
		return false, errOf(errSyntax, opName)
	}
	pos := 0
	if text[0] == '+' || text[0] == '-' {
		pos++
	}
	pos, digits, dotted, err := scanMantissa(text, pos, opName)
	if err != nil {
		return false, err
	}
	pos, exp, err := scanExponent(text, pos, opName)
	if err != nil {
		return false, err
	}
	if pos != len(text) || digits == 0 {
		return false, errOf(errSyntax, opName)
	}
	return dotted || exp, nil
}

func scanMantissa(text []byte, pos int, opName string) (int, int, bool, error) {
	pos, digits := scanDigits(text, pos)
	dotted := false
	if pos < len(text) && text[pos] == '.' {
		dotted = true
		pos++
		var more int
		pos, more = scanDigits(text, pos)
		digits += more
	}
	if digits == 0 {
		return pos, 0, false, errOf(errSyntax, opName)
	}
	return pos, digits, dotted, nil
}

func scanDigits(text []byte, pos int) (int, int) {
	start := pos
	for pos < len(text) && text[pos] >= '0' && text[pos] <= '9' {
		pos++
	}
	return pos, pos - start
}

func scanExponent(text []byte, pos int, opName string) (int, bool, error) {
	if pos >= len(text) || (text[pos] != 'e' && text[pos] != 'E') {
		return pos, false, nil
	}
	pos++
	if pos < len(text) && (text[pos] == '+' || text[pos] == '-') {
		pos++
	}
	var digits int
	pos, digits = scanDigits(text, pos)
	if digits == 0 {
		return pos, false, errOf(errSyntax, opName)
	}
	return pos, true, nil
}

func parseIntegerToken(text []byte, opName string) (float64, error) {
	value, err := strconv.ParseInt(string(text), decBase, bitSize64)
	if err != nil {
		return parseRealToken(text, opName)
	}
	return float64(value), nil
}

func parseRealToken(text []byte, opName string) (float64, error) {
	value, err := strconv.ParseFloat(string(text), bitSize64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) || math.IsInf(value, 0) {
			return 0, errOf(errRangecheck, opName)
		}
		return 0, errOf(errSyntax, opName)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errOf(errRangecheck, opName)
	}
	return value, nil
}

func opCvs(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return interp.Push(StringObj([]byte(cvsText(obj)), false))
}

func cvsText(obj Object) string {
	switch obj.Kind {
	case KindInt:
		return strconv.FormatInt(int64(obj.Int), decBase)
	case KindReal:
		return formatReal(obj.Real)
	case KindBool:
		return boolText(obj.Bool)
	case KindName:
		return obj.Name
	case KindNull, KindMark, KindString, KindArray, KindDict, KindOp:
		return noStringVal
	default:
		return noStringVal
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatReal(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return noStringVal
	}
	text := strconv.FormatFloat(value, 'g', shortestPrec, bitSize64)
	if strings.ContainsAny(text, ".eE") {
		return text
	}
	return text + ".0"
}

func opCvn(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	if obj.Kind != KindString || obj.Str == nil {
		return errOf(errTypecheck, "cvn")
	}
	text := string(obj.Str.Bytes)
	if obj.Exec || obj.Str.Exec {
		return interp.Push(ExecName(text))
	}
	return interp.Push(LiteralName(text))
}

func opCvx(ctx context.Context, interp *Interp) error {
	return setAccess(ctx, interp, true, "cvx")
}

func opCvlit(ctx context.Context, interp *Interp) error {
	return setAccess(ctx, interp, false, "cvlit")
}

func setAccess(ctx context.Context, interp *Interp, exec bool, opName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	updated, err := withExec(obj, exec, opName)
	if err != nil {
		return err
	}
	return interp.Push(updated)
}

func withExec(obj Object, exec bool, opName string) (Object, error) {
	var zero Object
	switch obj.Kind {
	case KindName:
		obj.Exec = exec
	case KindArray:
		obj.Exec = exec
		setArrExec(obj.Arr, exec)
	case KindString:
		obj.Exec = exec
		setStrExec(obj.Str, exec)
	case KindNull, KindBool, KindInt, KindReal, KindMark, KindDict, KindOp:
		return zero, errOf(errTypecheck, opName)
	default:
		return zero, errOf(errTypecheck, opName)
	}
	return obj, nil
}

func setArrExec(arr *Arr, exec bool) {
	if arr != nil {
		arr.Exec = exec
	}
}

func setStrExec(str *Str, exec bool) {
	if str != nil {
		str.Exec = exec
	}
}

func opLength(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	size, err := objectLen(obj)
	if err != nil {
		return err
	}
	return interp.Push(IntObj(size))
}

func objectLen(obj Object) (int32, error) {
	switch obj.Kind {
	case KindString:
		return stringLen(obj)
	case KindArray:
		return arrayLen(obj)
	case KindDict:
		return dictLen(obj)
	case KindName:
		return asInt32(len(obj.Name), "length")
	case KindNull, KindBool, KindInt, KindReal, KindMark, KindOp:
		return 0, errOf(errTypecheck, "length")
	default:
		return 0, errOf(errTypecheck, "length")
	}
}

func stringLen(obj Object) (int32, error) {
	if obj.Str == nil {
		return 0, nil
	}
	return asInt32(len(obj.Str.Bytes), "length")
}

func arrayLen(obj Object) (int32, error) {
	if obj.Arr == nil {
		return 0, nil
	}
	return asInt32(len(obj.Arr.Elems), "length")
}

func dictLen(obj Object) (int32, error) {
	if obj.Dict == nil {
		return 0, nil
	}
	return asInt32(obj.Dict.Len(), "length")
}

func opGet(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index, err := interp.Pop()
	if err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	got, err := getElem(obj, index)
	if err != nil {
		return err
	}
	return interp.Push(got)
}

func getElem(obj, index Object) (Object, error) {
	switch obj.Kind {
	case KindString:
		return getString(obj, index)
	case KindArray:
		return getArray(obj, index)
	case KindDict:
		return getDict(obj, index)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindMark, KindOp:
		return zeroObj(), errOf(errTypecheck, "get")
	default:
		return zeroObj(), errOf(errTypecheck, "get")
	}
}

func zeroObj() Object {
	var obj Object
	return obj
}

func indexAt(index Object, opName string) (int, error) {
	if index.Kind != KindInt {
		return 0, errOf(errTypecheck, opName)
	}
	if index.Int < 0 {
		return 0, errOf(errRangecheck, opName)
	}
	return int(index.Int), nil
}

func getString(obj, index Object) (Object, error) {
	if obj.Str == nil {
		return zeroObj(), errOf(errRangecheck, "get")
	}
	pos, err := indexAt(index, "get")
	if err != nil {
		return zeroObj(), err
	}
	if pos >= len(obj.Str.Bytes) {
		return zeroObj(), errOf(errRangecheck, "get")
	}
	return IntObj(int32(obj.Str.Bytes[pos])), nil
}

func getArray(obj, index Object) (Object, error) {
	if obj.Arr == nil {
		return zeroObj(), errOf(errRangecheck, "get")
	}
	pos, err := indexAt(index, "get")
	if err != nil {
		return zeroObj(), err
	}
	if pos >= len(obj.Arr.Elems) {
		return zeroObj(), errOf(errRangecheck, "get")
	}
	return obj.Arr.Elems[pos], nil
}

func getDict(obj, key Object) (Object, error) {
	if key.Kind != KindName {
		return zeroObj(), errOf(errTypecheck, "get")
	}
	if obj.Dict == nil {
		return zeroObj(), errOf(errUndefined, "get")
	}
	val, ok := obj.Dict.get(key.Name)
	if !ok {
		return zeroObj(), errOf(errUndefined, "get")
	}
	return val, nil
}

func opPut(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, err := interp.Pop()
	if err != nil {
		return err
	}
	index, err := interp.Pop()
	if err != nil {
		return err
	}
	obj, err := interp.Pop()
	if err != nil {
		return err
	}
	return putElem(obj, index, value)
}

func putElem(obj, index, value Object) error {
	switch obj.Kind {
	case KindString:
		return putString(obj, index, value)
	case KindArray:
		return putArray(obj, index, value)
	case KindDict:
		return putDict(obj, index, value)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindMark, KindOp:
		return errOf(errTypecheck, "put")
	default:
		return errOf(errTypecheck, "put")
	}
}

func putString(obj, index, value Object) error {
	if obj.Str == nil {
		return errOf(errRangecheck, "put")
	}
	pos, err := indexAt(index, "put")
	if err != nil {
		return err
	}
	if pos >= len(obj.Str.Bytes) {
		return errOf(errRangecheck, "put")
	}
	byteValue, err := putByte(value)
	if err != nil {
		return err
	}
	obj.Str.Bytes[pos] = byteValue
	return nil
}

func putByte(value Object) (byte, error) {
	if value.Kind != KindInt {
		return 0, errOf(errTypecheck, "put")
	}
	if value.Int < 0 || value.Int > maxByteValue {
		return 0, errOf(errRangecheck, "put")
	}
	return byte(value.Int), nil
}

func putArray(obj, index, value Object) error {
	if obj.Arr == nil {
		return errOf(errRangecheck, "put")
	}
	pos, err := indexAt(index, "put")
	if err != nil {
		return err
	}
	if pos >= len(obj.Arr.Elems) {
		return errOf(errRangecheck, "put")
	}
	obj.Arr.Elems[pos] = value
	return nil
}

func putDict(obj, key, value Object) error {
	if key.Kind != KindName {
		return errOf(errTypecheck, "put")
	}
	if obj.Dict == nil {
		return errOf(errTypecheck, "put")
	}
	obj.Dict.putNew(key.Name, value)
	return nil
}

func opNull(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(NullObj())
}
