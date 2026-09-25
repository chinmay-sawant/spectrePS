package font

import "fmt"

// t1TokenKind names the token shapes the Type 1 tokenizer recognizes.
type t1TokenKind uint8

const (
	t1Number t1TokenKind = iota
	t1Name
	t1LiteralName
	t1String
	t1ArrayOpen
	t1ArrayClose
	t1BraceOpen
	t1BraceClose
	t1RD
)

// t1Token is one PostScript token from a font program. An RD token carries
// the raw charstring bytes that follow the RD keyword.
type t1Token struct {
	kind t1TokenKind
	num  float64
	name string
	data []byte
}

// Token sizes the tokenizer allocates for.
const t1StringCapacity = 16

// type1Tokenize splits a cleartext or decrypted program into tokens. RD and
// the -| spelling consume a fixed byte run, so binary charstring bytes never
// reach the scanner.
func type1Tokenize(src []byte) ([]t1Token, error) {
	lexer := &t1Lexer{src: src, pos: 0, tokens: nil}
	if err := lexer.run(); err != nil {
		return nil, err
	}
	return lexer.tokens, nil
}

// t1Lexer walks one byte slice and appends tokens.
type t1Lexer struct {
	src    []byte
	pos    int
	tokens []t1Token
}

// run tokenizes until the input ends.
func (lexer *t1Lexer) run() error {
	for lexer.pos < len(lexer.src) {
		if err := lexer.step(); err != nil {
			return err
		}
	}
	return nil
}

// step consumes one token or one byte of whitespace.
func (lexer *t1Lexer) step() error {
	b := lexer.src[lexer.pos]
	switch {
	case isType1Space(b):
		lexer.pos++
		return nil
	case b == '%':
		lexer.skipComment()
		return nil
	case b == '/':
		return lexer.literalName()
	case b == '(':
		return lexer.string()
	case b == '<':
		return lexer.hexString()
	case isType1Delimiter(b):
		return lexer.delimiter(b)
	case isType1NumberStart(lexer.src, lexer.pos):
		return lexer.number()
	default:
		return lexer.name()
	}
}

// isType1Delimiter reports whether the byte is one of the bracket or brace
// delimiters the tokenizer records.
func isType1Delimiter(b byte) bool {
	return b == '[' || b == ']' || b == '{' || b == '}'
}

// isType1NumberStart reports whether a number token starts at pos. The -|
// spelling is an executable name, not a number.
func isType1NumberStart(src []byte, pos int) bool {
	b := src[pos]
	switch {
	case b == '-':
		return !(pos+1 < len(src) && src[pos+1] == '|')
	case b == '+' || b == '.':
		return true
	default:
		return b >= '0' && b <= '9'
	}
}

// delimiter emits one bracket or brace token.
func (lexer *t1Lexer) delimiter(b byte) error {
	var kind t1TokenKind
	switch b {
	case '[':
		kind = t1ArrayOpen
	case ']':
		kind = t1ArrayClose
	case '{':
		kind = t1BraceOpen
	default:
		kind = t1BraceClose
	}
	lexer.pos++
	lexer.tokens = append(lexer.tokens, t1Token{kind: kind, num: 0, name: "", data: nil})
	return nil
}

// skipComment drops a % comment through the next line ending.
func (lexer *t1Lexer) skipComment() {
	for lexer.pos < len(lexer.src) && lexer.src[lexer.pos] != '\n' && lexer.src[lexer.pos] != '\r' {
		lexer.pos++
	}
}

// readRegular consumes the regular characters at the cursor.
func (lexer *t1Lexer) readRegular() string {
	start := lexer.pos
	for lexer.pos < len(lexer.src) && isType1Regular(lexer.src[lexer.pos]) {
		lexer.pos++
	}
	return string(lexer.src[start:lexer.pos])
}

// literalName reads a /Name token.
func (lexer *t1Lexer) literalName() error {
	lexer.pos++
	name := lexer.readRegular()
	if name == "" {
		return fmt.Errorf("%w: empty literal name", errType1Syntax)
	}
	lexer.tokens = append(lexer.tokens, t1Token{kind: t1LiteralName, num: 0, name: name, data: nil})
	return nil
}

// name reads an executable name and turns RD or -| into a byte-run token.
func (lexer *t1Lexer) name() error {
	start := lexer.pos
	name := lexer.readRegular()
	if name == "" {
		return fmt.Errorf("%w: unexpected byte %q", errType1Syntax, lexer.src[start])
	}
	if name == "RD" || name == "-|" {
		if last := len(lexer.tokens) - 1; last >= 0 && lexer.tokens[last].kind == t1Number {
			return lexer.readRD(int(lexer.tokens[last].num))
		}
	}
	lexer.tokens = append(lexer.tokens, t1Token{kind: t1Name, num: 0, name: name, data: nil})
	return nil
}

// readRD consumes the byte run after an RD keyword. The scanner already
// consumed the delimiter of the RD token when one byte of whitespace follows.
func (lexer *t1Lexer) readRD(count int) error {
	if count < 0 {
		return fmt.Errorf("%w: negative RD length", errType1Syntax)
	}
	if lexer.pos < len(lexer.src) && isType1Space(lexer.src[lexer.pos]) {
		lexer.pos++
	}
	if lexer.pos+count > len(lexer.src) {
		return fmt.Errorf("%w: RD run of %d bytes", errType1Truncated, count)
	}
	data := make([]byte, count)
	copy(data, lexer.src[lexer.pos:lexer.pos+count])
	lexer.pos += count
	lexer.tokens = append(lexer.tokens, t1Token{kind: t1RD, num: 0, name: "", data: data})
	return nil
}

// number reads an integer or real token.
func (lexer *t1Lexer) number() error {
	text := lexer.readRegular()
	value, ok := t1ParseNumber(text)
	if !ok {
		return fmt.Errorf("%w: number %q", errType1Syntax, text)
	}
	lexer.tokens = append(lexer.tokens, t1Token{kind: t1Number, num: value, name: "", data: nil})
	return nil
}

// string reads a parenthesized string with backslash escapes.
func (lexer *t1Lexer) string() error {
	lexer.pos++
	depth := 1
	text := make([]byte, 0, t1StringCapacity)
	for lexer.pos < len(lexer.src) {
		b := lexer.src[lexer.pos]
		lexer.pos++
		switch b {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				lexer.tokens = append(lexer.tokens, t1Token{kind: t1String, num: 0, name: "", data: text})
				return nil
			}
		case '\\':
			text = lexer.escape(text)
			continue
		}
		text = append(text, b)
	}
	return fmt.Errorf("%w: unterminated string", errType1Truncated)
}

// escape appends the escaped byte or bytes at the cursor.
func (lexer *t1Lexer) escape(text []byte) []byte {
	if lexer.pos >= len(lexer.src) {
		return append(text, '\\')
	}
	b := lexer.src[lexer.pos]
	lexer.pos++
	switch b {
	case 'n':
		return append(text, '\n')
	case 'r':
		return append(text, '\r')
	case 't':
		return append(text, '\t')
	case 'b':
		return append(text, '\b')
	case 'f':
		return append(text, '\f')
	}
	return append(text, b)
}

// hexString reads a <...> hex string. An odd final digit is padded with 0.
func (lexer *t1Lexer) hexString() error {
	lexer.pos++
	var digits []byte
	for lexer.pos < len(lexer.src) {
		b := lexer.src[lexer.pos]
		lexer.pos++
		if b == '>' {
			if len(digits)%type1DigitsPerByte != 0 {
				digits = append(digits, '0')
			}
			data, ok := type1HexText(digits)
			if !ok {
				return fmt.Errorf("%w: hex string", errType1Syntax)
			}
			lexer.tokens = append(lexer.tokens, t1Token{kind: t1String, num: 0, name: "", data: data})
			return nil
		}
		if isType1HexDigit(b) {
			digits = append(digits, b)
		}
	}
	return fmt.Errorf("%w: unterminated hex string", errType1Truncated)
}
