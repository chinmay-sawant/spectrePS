package pdf

import "reflect"

// maxMarkedDepth caps marked-content nesting. The 65th open sequence is
// limitcheck.
const maxMarkedDepth = 64

// mcFrame is one open marked-content sequence.
type mcFrame struct {
	tag   string
	depth int
}

// takeMarked dispatches the five marked-content operators.
func (run *runner) takeMarked(opName string) (bool, error) {
	switch opName {
	case "BMC":
		return true, run.beginMarked("BMC", false)
	case "BDC":
		return true, run.beginMarked("BDC", true)
	case "MP":
		return true, run.pointMarked("MP", false)
	case "DP":
		return true, run.pointMarked("DP", true)
	case "EMC":
		return true, run.endMarked()
	default:
		return false, nil
	}
}

// beginMarked pops a tag and, for BDC, the properties operand, then opens one
// nesting level. The depth passed to the sink is 1 for the outermost sequence.
// A sequence whose optional content group is OFF opens a skip instead, so its
// painting operators never run. The frame and the sink event stay paired.
func (run *runner) beginMarked(opName string, withProps bool) error {
	props := NullVal()
	raw := otherItem()
	if withProps {
		got, err := run.popPropertyItem(opName)
		if err != nil {
			return err
		}
		raw = got
		resolved, err := run.propertyValue(raw, opName)
		if err != nil {
			return err
		}
		props = resolved
	}
	tag, err := run.popName(opName)
	if err != nil {
		return err
	}
	hidden := false
	if withProps {
		hidden, err = run.ocHidden(props, raw)
		if err != nil {
			return err
		}
	}
	if len(run.mcStack) >= maxMarkedDepth {
		return NewError(opName, errLimit)
	}
	depth := len(run.mcStack) + 1
	run.mcStack = append(run.mcStack, mcFrame{tag: tag, depth: depth})
	if activeSink(run.mcSink) {
		run.mcSink.BeginMarkedContent(tag, props, depth)
	}
	if hidden {
		run.skipDepth = 1
	}
	return nil
}

// pointMarked pops the operands of MP or DP. A point has no EMC, so it fires
// no sink event and changes no nesting depth.
func (run *runner) pointMarked(opName string, withProps bool) error {
	if withProps {
		if _, err := run.popProperty(opName); err != nil {
			return err
		}
	}
	_, err := run.popName(opName)
	return err
}

// endMarked closes the innermost sequence at its Begin depth. An unmatched EMC
// is syntaxerror in content.
func (run *runner) endMarked() error {
	count := len(run.mcStack)
	if count == 0 {
		return contentSyntax()
	}
	frame := run.mcStack[count-1]
	run.mcStack = run.mcStack[:count-1]
	if activeSink(run.mcSink) {
		run.mcSink.EndMarkedContent(frame.depth)
	}
	return nil
}

// popProperty pops the properties operand of BDC or DP and resolves it. A
// name resolves in the /Properties resources and a dictionary stands for
// itself. Anything else is typecheck and a name outside /Properties is
// undefined.
func (run *runner) popProperty(opName string) (Value, error) {
	raw, err := run.popPropertyItem(opName)
	if err != nil {
		return NullVal(), err
	}
	return run.propertyValue(raw, opName)
}

// popPropertyItem pops the raw properties operand and keeps its name text or
// dictionary, so the optional content check can read an indirect reference
// before resolution loses the object number.
func (run *runner) popPropertyItem(opName string) (item, error) {
	count := len(run.stack)
	if count == 0 {
		return otherItem(), NewError(opName, errUnderflow)
	}
	last := run.stack[count-1]
	if last.kind == itemName || last.kind == itemDict {
		run.stack = run.stack[:count-1]
		return last, nil
	}
	return otherItem(), NewError(opName, errType)
}

// propertyValue resolves one raw properties operand through /Properties or
// through the file. A name outside /Properties is undefined in opName.
func (run *runner) propertyValue(raw item, opName string) (Value, error) {
	if raw.kind == itemName {
		prop, ok := run.properties[raw.name]
		if !ok {
			return NullVal(), NewError(opName, errUndefined)
		}
		return run.derefValue(prop, opName)
	}
	return run.derefValue(raw.val, opName)
}

// derefValue resolves one resource value through the file when indirect.
func (run *runner) derefValue(val Value, opName string) (Value, error) {
	if run.file == nil || val.Kind != KindRef {
		return val, nil
	}
	resolved, err := run.file.deref(val)
	if err != nil {
		return NullVal(), NewError(opName, errUndefined)
	}
	return resolved, nil
}

// activeSink reports whether an optional interface holds a usable value. A nil
// interface and a typed-nil pointer both count as absent.
func activeSink(val any) bool {
	if val == nil {
		return false
	}
	ref := reflect.ValueOf(val)
	kind := ref.Kind()
	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface ||
		kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice {
		return !ref.IsNil()
	}
	return true
}
