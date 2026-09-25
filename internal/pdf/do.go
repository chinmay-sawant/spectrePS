package pdf

// opDo is the content operator name for one XObject paint.
const opDo = "Do"

// takeDo dispatches the Do operator.
func (run *runner) takeDo(opName string) (bool, error) {
	if opName != opDo {
		return false, nil
	}
	return true, run.do()
}

// do pops the name operand for one XObject paint.
// A non-name operand is typecheck and a missing operand is stackunderflow,
// both from popName. Until the image path lands, every name is undefined.
func (run *runner) do() error {
	if _, err := run.popName(opDo); err != nil {
		return err
	}
	return NewError(opDo, errUndefined)
}
