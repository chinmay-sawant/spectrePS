package ps

import "context"

func registerBannedOps(interp *Interp) {
	interp.Install("file", bannedOp("file"))
	interp.Install("run", bannedOp("run"))
	interp.Install("deletefile", bannedOp("deletefile"))
	interp.Install("renamefile", bannedOp("renamefile"))
	interp.Install("filenameforall", bannedOp("filenameforall"))
}

// bannedOp returns invalidaccess and does not read a path or the operand stack.
func bannedOp(name string) Operator {
	return func(ctx context.Context, _ *Interp) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errOf(errInvalidAccess, name)
	}
}
