package ps

import "context"

const (
	opLanguageLevel = "languagelevel"
	opVersion       = "version"
	opRevision      = "revision"
	opProduct       = "product"
	opSerialNumber  = "serialnumber"

	// languageLevel is the PostScript level whose operator set this package
	// implements. It is Level 1 plus the Level 2 operators listed in
	// documentation/language.md.
	languageLevel = 2
	// interpreterVersion mirrors the release tag spectreps.Version returns.
	// Bump the two together.
	interpreterVersion = "0.0.3"
	// interpreterProduct names this interpreter.
	interpreterProduct = "Spectre PS"
	// interpreterRevision is the sub-release integer. Spectre releases on tags
	// only, so it stays 0.
	interpreterRevision = 0
	// interpreterSerial is the interpreter serial number. Spectre has no
	// licensed serial number, so it stays 0.
	interpreterSerial = 0
)

// registerInfoOps installs the interpreter identification operators. The
// CUPS test page reads these to label itself; they are data, not graphics.
func registerInfoOps(interp *Interp) {
	interp.Install(opLanguageLevel, opLanguageLevelRun)
	interp.Install(opVersion, opVersionRun)
	interp.Install(opRevision, opRevisionRun)
	interp.Install(opProduct, opProductRun)
	interp.Install(opSerialNumber, opSerialNumberRun)
}

func opLanguageLevelRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(IntObj(languageLevel))
}

func opVersionRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(StringObj([]byte(interpreterVersion), false))
}

func opRevisionRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(IntObj(interpreterRevision))
}

func opProductRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(StringObj([]byte(interpreterProduct), false))
}

func opSerialNumberRun(ctx context.Context, interp *Interp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return interp.Push(IntObj(interpreterSerial))
}
