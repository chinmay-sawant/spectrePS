# PostScript subset

This is the contract for tag 0.0.2. A program that stays inside this file runs. A name this file does not list returns `undefined`, except the banned operators, which return `invalidaccess`.

The input is PostScript source bytes. Spectre does not invent a second page-description language. PDF content operators are specified in `documentations/devices.md`, because they are a different syntax on the same graphics engine.

## Tokens

The scanner recognizes:

- Whitespace, ignored.
- Comments from `%` through the end of the line. A header such as `%!PS-Adobe-3.0` is a comment. It is not required.
- Integers, optional sign, then digits. The value must fit in int32. A token that does not fit int32 and has no decimal point is a `rangecheck` at scan time.
- Reals: a decimal point or an exponent. Stored as float64.
- Executable names.
- Literal names introduced by `/`.
- Strings in parentheses, with escapes `\n`, `\r`, `\t`, `\\`, `\(`, `\)`, and `\ddd` octal. Strings are bytes. `length` counts bytes.
- Hex strings `<...>` with whitespace ignored. An odd final nibble is padded with 0.
- `{` and `}` as procedure delimiters, scanned by the reader, not executed as operators.

Delimiter characters are `()<>[]{}/%` plus whitespace. Names are case-sensitive. `moveto` and `MoveTo` are different names.

`[` `]` `<<` `>>` are operators, not scanner structure. `[` pushes a mark. `]` builds an array from the mark. `<<` pushes a mark. `>>` builds a dictionary.

## Objects

Simple objects, copied by value: null, boolean, int32, float64, name, mark.

Composite objects, shared by pointer: string, array, dictionary. `eq` on a composite is pointer equality. `eq` on numbers treats int 1 and real 1.0 as equal. Names are equal when the name text is equal.

An array or a name has an executable bit. `/foo` pushes a literal name. The token `foo` is an executable name.

## Stacks and execution

Three stacks:

- Operand stack, max 8192.
- Dictionary stack, max 20. Starts as `systemdict` under `userdict`. `systemdict` rejects `def` with `invalidaccess`. `userdict` accepts `def`.
- Execution stack, max 500.

Startup installs the operators in this file into `systemdict`.

Execution rule:

- An executable name is looked up from the top of the dictionary stack downward. The value is then executed. Lookup happens at execution time, not when `{` scans the procedure.
- An operator runs.
- An executable array met directly, including a procedure element inside another procedure, is pushed.
- An executable array met because `exec`, `if`, `ifelse`, or `repeat` invoked it is called. Calling pushes it on the execution stack and walks its elements.
- Any other object is pushed.

The reader turns `{ ... }` into one executable array and hands that array to the rule above, which pushes it. Top-level `1 2 add` runs `add` immediately because `add` is an executable name.

`store` walks the dictionary stack and replaces the first existing definition. If the name is missing, `store` defines it in the current dictionary. `load` returns the value without executing it. `where` pushes the dictionary and true, or false.

Dictionary `forall` walks entries in insertion order so tests are stable. PostScript leaves that order unspecified. This is a deliberate Spectre choice.

`for` accepts int or real start, increment, and limit, and calls the procedure once per value, matching the usual PostScript loop. A zero increment is `rangecheck`.

`exit` leaves the innermost `loop`, `repeat`, or `for`. `exit` outside those is `invalidexit`.

## Operators

Stack: `pop` `dup` `exch` `index` `roll` `clear` `count` `mark` `cleartomark` `counttomark`. `copy` is the count form only. A non-integer top operand to `copy` is `typecheck` in this tag. The composite destination form of `copy` is deferred.

Math: `add` `sub` `mul` `div` `idiv` `mod` `neg` `abs` `ceiling` `floor` `round` `sqrt`. `div` always pushes a real. `idiv` pushes an int32. Division by zero is `undefinedresult`. Integer overflow is `rangecheck`.

Compare and logic: `eq` `ne` `gt` `ge` `lt` `le` `and` `or` `not` `xor` `true` `false`.

Types: `type` `xcheck` `cvi` `cvr` `cvs` `cvn` `cvx` `cvlit` `length` `get` `put`.

Arrays and dictionaries: `array` `dict` `def` `load` `store` `where` `known` `begin` `end` `[` `]` `<<` `>>`.

Control: `exec` `if` `ifelse` `repeat` `for` `loop` `forall` `exit`.

Path and paint: `moveto` `rmoveto` `lineto` `rlineto` `curveto` `rcurveto` `closepath` `newpath` `currentpoint` `stroke` `fill` `eofill` `setlinewidth` `setrgbcolor` `setgray` `gsave` `grestore` `showpage`.

Graphics defaults: line width 1, line cap 0, line join 0, miter limit 10, solid dash, gray 0. `gsave` depth max 32. Path point max 100000.

`showpage` finishes the current page and starts a blank one. If the program paints and never calls `showpage`, the job finishes one page at the end. If it paints nothing and never calls `showpage`, the job still finishes one blank page.

Matrix operators `translate` `scale` `rotate` `concat` `setmatrix` `currentmatrix` are in this tag. The default matrix is the identity in user points, origin lower left, one unit equal to one point.

## Errors

| Name | When |
| --- | --- |
| `stackunderflow` | An operator needs more operands. |
| `stackoverflow` | The operand stack cap is hit. |
| `typecheck` | An operand has the wrong kind. |
| `undefined` | A name is missing, and it is not a banned operator. |
| `rangecheck` | A number, index, or page size is outside the allowed range. |
| `undefinedresult` | Division by zero, or `sqrt` of a negative. |
| `unmatchedmark` | `]` or `cleartomark` finds no mark. |
| `syntaxerror` | The scanner rejects the bytes, or braces do not match. |
| `limitcheck` | Exec stack, dict stack, path, gsave, or pixel cap. |
| `invalidaccess` | Write to `systemdict`, or a banned operator. |
| `invalidexit` | `exit` with no loop. |
| `dictstackunderflow` | `end` when only `systemdict` remains. |

## Banned operators

These names are defined and return `invalidaccess`: `file` `run` `deletefile` `renamefile` `filenameforall`.

Pipe paths such as `%pipe%...` never run, because `file` returns `invalidaccess` before it reads the path. No operator in this tag opens a network connection or starts a process.

## Limits

| Cap | Value |
| --- | --- |
| Operand stack | 8192 |
| Execution stack | 500 |
| Dictionary stack | 20 |
| `gsave` depth | 32 |
| Procedure nesting while scanning | 128 |
| Path points | 100000 |
| Pixels per page | 40000000 |
| Side of a page, pixels | 20000 |

A default letter page at 72 dpi is 612 by 792 pixels. A letter page at 300 dpi is under the pixel cap. A letter page at 600 dpi is over the cap and returns `limitcheck`.

## Out of this tag

Fonts, `show`, images, `clip`, `save`, `restore`, `bind`, filters, and file I/O. `bind` is omitted on purpose, so a name inside a procedure sees the definition from execution time. A test must redefine a name after building a procedure and observe the new value.
