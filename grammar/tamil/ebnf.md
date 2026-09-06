# Niraluli Tamil EBNF

Notation: same as Go (`grammar/go/ebnf-notation.md`).
Keywords: `grammar/tamil/keywords.yaml`.
Status: **Tamil-0.77** (2026-09-05) — var groups, bitwise, int widths, generic aliases, typed fmt.

```
SourceFile    = PackageClause { ImportDecl } { TopLevelDecl } .
PackageClause = "தொகுப்பு" PackageName .
PackageName   = identifier .
ImportDecl    = "கொணர்" [ identifier ] string_lit .

TopLevelDecl  = [ "வெளி" ] ( FunctionDecl | ConstDecl | ConstGroupDecl | VarDecl | VarGroupDecl | TypeDecl ) .

TypeDecl   = "வகை" identifier ( "=" Type | TypeLit | Type ) .
TypeLit    = StructType .
StructType = "அமைப்பு" "{" { FieldDecl } "}" .
FieldDecl  = [ "வெளி" ] identifier Type .

FunctionDecl  = "செயல்பாடு" [ Receiver ] FunctionName [ TypeParams ] Signature FunctionBody .
Receiver      = "(" identifier Type ")" .
FunctionName  = identifier .
TypeParams    = "[" TypeParam { "," TypeParam } "]" .
TypeParam     = identifier [ Constraint ] .
Constraint    = Type { "|" Type } | "எதுவும்" .
Signature     = "(" [ ParameterList ] ")" [ Result ] .
ParameterList = ParameterDecl { "," ParameterDecl } .
ParameterDecl = identifier [ "..." ] Type .
Result        = Type | "(" [ ResultList ] ")" .
ResultList    = NamedResults | TypeList .
NamedResults  = identifier Type { "," identifier Type } .
TypeList      = [ "..." ] Type { "," [ "..." ] Type } .
FunctionBody  = Block .

Type          = TypeName | SliceType | ArrayType | PointerType | MapType | FuncType .
ArrayType     = "[" integer_lit "]" Type .
SliceType     = "[" "]" Type .
MapType       = "அகராதி" "[" Type "]" Type .
FuncType      = "செயல்பாடு" "(" [ TypeList ] ")" [ Result ] .
TypeName      = "முழுஎண்" | "முழுஎண்8" | "முழுஎண்16" | "முழுஎண்32" | "முழுஎண்64"
              | "நேர்முழு8" | "நேர்முழு16" | "நேர்முழு32" | "நேர்முழு64"
              | "நிலை" | "சரம்" | "மிதவைஎண்" | "இருமி8" | "இருமி32" | QualifiedName .
QualifiedName = [ identifier "." ] identifier .
PointerType   = "*" Type .

VarDecl        = "மாறி" ( VarSpec | "(" { VarSpec ";" } ")" ) .
VarSpec        = IdentifierList ( Type [ "=" ExpressionList ] | "=" ExpressionList ) .
VarGroupDecl   = "மாறி" "(" VarSpec { ";" VarSpec } ")" .
ConstDecl      = "மாறிலி" IdentifierList [ Type ] "=" ExpressionList .
ConstGroupDecl = "மாறிலி" "(" ConstSpec { ";" ConstSpec } ")" .
ConstSpec      = IdentifierList [ Type ] [ "=" ExpressionList ] .
ShortVarDecl  = IdentifierList ":=" ExpressionList .
IdentifierList = identifier { "," identifier } .
ExpressionList = Expression { "," Expression } .

Block         = "{" StatementList "}" .
StatementList = { Statement } .

Statement     = ConstDecl | ConstGroupDecl | VarDecl | VarGroupDecl | SimpleStmt | IfStmt | SwitchStmt | ForStmt | BreakStmt | ContinueStmt | ReturnStmt | DeferStmt | Block .
DeferStmt     = "தள்ளிவை" ( CallExpr | identifier | SelectorExpr ) .
SimpleStmt    = ExpressionStmt | Assignment | ShortVarDecl .
ExpressionStmt = Expression .
Assignment     = ExpressionList "=" ExpressionList .

IfStmt = "எனில்" [ SimpleStmt ";" ] Expression Block [ "இல்லையேல்" ( IfStmt | Block ) ] .

SwitchStmt = "திசைவி" [ SimpleStmt ";" ] [ Expression ] "{" { CaseClause } "}" .
CaseClause = "எனில்" ExpressionList Block | "மற்றபடி" Block .

ForStmt     = "சுழல்" [ Condition | ForClause | RangeClause ] Block .
Condition   = Expression .
ForClause   = [ SimpleStmt ] ";" [ Expression ] ";" [ SimpleStmt ] .
RangeClause = [ IdentifierList ( ":=" | "=" ) ] "ஒவ்வொரு" Expression .

BreakStmt = "முறி" .
ContinueStmt = "தொடர்" .
ReturnStmt = "திருப்பு" [ ExpressionList ] .

Expression = Equality .
Equality   = Comparison { ( "==" | "!=" ) Comparison } .
Comparison = Term { ( "<" | "<=" | ">" | ">=" ) Term } .
Term       = Factor { ( "+" | "-" | "|" | "^" ) Factor } .
Factor     = Unary { ( "*" | "/" | "%" | "<<" | ">>" | "&" | "&^" ) Unary } .
Unary      = ( "-" | "!" | "*" | "&" | "^" ) Unary | Primary .
Primary    = Operand { IndexOrSlice | "." identifier | Arguments } .
IndexOrSlice = "[" Expression "]" | "[" [ Expression ] ":" [ Expression ] "]" .
Arguments  = "(" [ ExpressionList ] ")" .
Operand    = identifier | integer_lit | string_lit | "மெய்" | "பொய்" | "இன்மை"
           | CompositeLit | BuiltinCall | Call | FuncLit | "(" Expression ")" .
FuncLit    = "செயல்பாடு" "(" [ ParameterList ] ")" [ Result ] Block .

CompositeLit = ( SliceType | ArrayType | MapType | TypeName ) "{" [ LiteralElement { "," LiteralElement } ] "}" .
LiteralElement = Expression | Expression ":" Expression .
IndexExpr    = Primary "[" Expression "]" .
SliceExpr    = Primary "[" [ Expression ] ":" [ Expression ] "]"
             | Primary "[" [ Expression ] ":" Expression ":" Expression "]" .
SelectorExpr = Primary "." identifier .

BuiltinCall = ( "பதிப்பி" | "நீளம்" | "திறன்" ) "(" Expression ")"
            | "சேர்" "(" Expression { "," Expression } ")"
            | "நகல்" "(" Expression "," Expression ")"
            | "நீக்கு" "(" Expression "," Expression ")"
            | "ஆக்கு" "(" Type [ "," Expression [ "," Expression ] ] ")" .
Call        = Primary "(" [ ExpressionList ] ")" .

identifier  = letter { letter | decimal_digit | unicode_mark } .
letter      = unicode_letter | "_" .
unicode_mark = /* Unicode category Mn|Mc|Me */ .
integer_lit = decimal_digit { decimal_digit } .
string_lit  = /* UTF-8 string in double quotes */ .
```

## Semicolons (Go-style)

The lexer inserts a `;` after a line's final token when that token is an
identifier, integer/string/bool/`இன்மை` literal, `முறி` / `தொடர்` / `திருப்பு`,
a type keyword (`முழுஎண்` / `நிலை` / `சரம்`), or `)` `]` `}`.
Explicit `;` remains valid (e.g. `சுழல்` clauses).

## Comments (Go-style)

```
// line comment — to end of line
/* general comment — to closing */
```

## Multi-file packages (Tamil-0.9)

Several `SourceFile`s with the same `PackageName` form one package.
Decls are shared (types, funcs, methods). Exactly one entry
`செயல்பாடு தொடக்கம்()` across the package. Tooling: `uli <dir>` or
`uli a.uli b.uli …`. Cross-package: `கொணர் "rel/dir"` then `pkg.Name`.

## Export (Tamil-0.11)

Names are package-private by default. Prefix `வெளி` on a top-level
func/type or struct field to make it visible to importers. Same package
always sees unmarked names. Replaces Go's capitalization rule.

## Nil (Tamil-0.12)

`இன்மை` is the nil pointer (C `NULL`). Use in pointer contexts and in
`==` / `!=` with pointers. Short declaration `p := இன்மை` is rejected
(no type to infer).

## Struct fields (Tamil-0.13)

Field types may be scalars, named structs (by value), `*T` (incl. recursive
`*Self`), or `[]முழுஎண்` / `[]நிலை` / `[]சரம்`. A struct must not contain
itself by value (pointer cycles are fine).

## Switch (Tamil-0.14)

`திசைவி` [tag] `{ எனில் … | மற்றபடி … }`. Cases reuse `எனில்`; no fallthrough.
Tagless form uses boolean conditions. See `constructs/switch.yaml`.

## If / switch init (Tamil-0.15)

Optional `SimpleStmt ;` before the `எனில்` condition or `திசைவி` tag
(Go-style). Init names are in scope for the condition/tag, body, cases,
and `இல்லையேல்`. See `constructs/if-init.yaml`.

## Struct equality (Tamil-0.16)

`==` / `!=` on struct values when all fields are comparable (no slice
fields). See `constructs/struct-eq.yaml`.

## Print structs (Tamil-0.17)

`பதிப்பி(s)` for a named struct prints `Type{field: value, …}`. See
`constructs/print-struct.yaml`.

## Import aliases (Tamil-0.18)

`கொணர் alias "path"` binds `alias` as the qualifier; omit alias to use the
imported `தொகுப்பு` name. `கொணர் _ "path"` is blank (no qualifier). See
`constructs/import-alias.yaml`.

## Slice growth (Tamil-0.19)

`சேர்(xs, a, b, …)` appends multiple elements. `xs[i:j:k]` sets capacity
(`k` is max index). See `constructs/slice-grow.yaml`.

## Positional struct literals (Tamil-0.20)

`புள்ளி{3, 4}` fills fields in declaration order (no mix with keyed). See
`constructs/struct-lit-pos.yaml`.

## Nested slices (Tamil-0.21)

`[][]முழுஎண்` and deeper nesting. See `constructs/nested-slice.yaml`.

## Make / copy / cap (Tamil-0.22)

`ஆக்கு`, `நகல்`, `திறன்`. See `constructs/make-copy.yaml`.

## Multiple return values (Tamil-0.23)

`(T, U)` results; unpack with `:=` / `=`. See `constructs/multi-return.yaml`.

## Fixed arrays (Tamil-0.24)

`[N]T` value arrays. See `constructs/array.yaml`.

## Slice of struct / pointer (Tamil-0.25)

`[]புள்ளி`, `[]*முழுஎண்`. See `constructs/slice-struct.yaml`.

## Named results (Tamil-0.26)

`(ஈ முழுஎண், மீ முழுஎண்)`. See `constructs/named-results.yaml`.

## Naked return (Tamil-0.27)

Bare `திருப்பு` returns named result locals. See `constructs/naked-return.yaml`.

## Parallel assign (Tamil-0.28)

`அ, ஆ = ஆ, அ`. See `constructs/parallel-assign.yaml`.

## Type alias / defined (Tamil-0.29)

`வகை எண் = முழுஎண்`, `வகை மதிப்பெண் முழுஎண்`. See `constructs/type-alias.yaml`.

## Floats (Tamil-0.30)

`மிதவைஎண்`, `1.5`. See `constructs/float.yaml`.

## Tamil digits (Tamil-0.31)

`௧௨` in literals. See `constructs/tamil-digits.yaml`.

## Unused imports (Tamil-0.32)

Hard error + clearer cycle paths. See `constructs/import-unused.yaml`.

## Conversions (Tamil-0.33)

`மதிப்பெண்(x)`, `முழுஎண்(3.9)`, `மிதவைஎண்(n)`. See `constructs/conversion.yaml`.

## Conversion extras (Tamil-0.34)

`சரம்(65)`, `(*T)(p)`, mixed `2 + 3.5`. See `constructs/conversion-extra.yaml`.

## Maps (Tamil-0.35)

`அகராதி[K]V`, `ஆக்கு`, index get/set, comma-ok, `நீளம்`, `ஒவ்வொரு`,
`நீக்கு`, `இன்மை`. See `constructs/map.yaml`.

## Map literals (Tamil-0.36)

`அகராதி[K]V{k: v, …}` and `{}` (non-nil empty). See `constructs/map-literal.yaml`.

## Defer (Tamil-0.37 / 0.42 / 0.43)

`தள்ளிவை` call or bare name/selector; pointer methods save `&x`.
See `constructs/defer.yaml`.

## Richer map keys (Tamil-0.38)

Comparable keys: floats, pointers, structs, arrays. See `constructs/map-keys.yaml`.

## Byte / rune (Tamil-0.39)

`இருமி8`, `இருமி32`, `[]இருமி8` / `[]இருமி32` ↔ `சரம்`. See `constructs/byte-rune.yaml`.

## Generics (Tamil-0.40 / 0.41 / 0.71 / 0.72)

Unconstrained function type params `[யா, ஆ]`; optional union constraints
`[T முழுஎண் | சரம்]` or explicit `எதுவும்` (any). Tamil-0.71 adds generic
`வகை` declarations and `Name[T]` instantiation (structs and defined types).
Tamil-0.77 adds methods on generic types: receiver uses the type's parameters
(`(b பெட்டி[யா])`), monomorphized per instantiation. Inference or `f[T](…)` /
`f[T,U](…)`; exported via `கொணர்`. See `constructs/generics.yaml`.

## Function values (Tamil-0.44)

`செயல்பாடு(TypeList) [Result]` is a type. Bare `X.M` (method) and bare
package function names are values; call through a function-typed
expression. Receiver bind matches defer 0.43. No function literals.
See `constructs/func-value.yaml`.

## Closures (Tamil-0.45 / 0.46)

`செயல்பாடு(params) [results] { body }` as an expression. Captures free
variables by reference (arena-promoted locals in C). Tamil-0.46: `சுழல்`
/ `ஒவ்வொரு` iteration variables (`:=`) are per-iteration (Go 1.22+).
See `constructs/closure.yaml`.

## Method expressions (Tamil-0.47)

`Type.Method` and `(*Type).Method` yield unbound `செயல்பாடு` values with
the receiver as the first parameter. Method-set rules match Go.
See `constructs/method-expr.yaml`.

## Concurrency (Tamil-0.48–0.51)

`இழை`, `தடம்`, `<-`, `மூடு`, `தடத்தேர்வு` / `எனில்` / `மற்றபடி` on the C
backend (pthread). Select cases reuse `எனில்` (same as `திசைவி`).
Tamil-0.51: `சுழல் ஒவ்வொரு` over `தடம்` (recv until `மூடு`). See
`constructs/goroutine.yaml`, `range.yaml`.

## Panic / recover (Tamil-0.49–0.50)

Builtins `அலறு(…)` and `மீள்()` (returns `சரம்`). Unwind runs
`தள்ளிவை` first. Tamil-0.50: primitive `அலறு` args, uncaught stack
dump, per-`இழை` state. See `constructs/panic.yaml`. C backend:
`setjmp` / `longjmp`.

## Heap / GC (Tamil-0.52)

No new syntax. C backend uses a stop-the-world conservative mark-sweep
heap instead of a bump arena. See `constructs/gc.yaml`.

## TCP / UDP sockets (Tamil-0.61)

No new syntax. Stdlib package `வலை` (`கொணர் "வலை"`): `கேள்` / `ஏற்று` /
`இணை` / `படி` / `எழுது` / `விடு` / `நிறுத்து` / `முகவரி`; UDP
`தகவல்கேள்` / `தகவல்முகவரி` / `தகவலனுப்பு` / `தகவல்பெறு` /
`தகவல்விடு`. IPv4, Linux C backend. See `constructs/net.yaml`.

## HTTP (Tamil-0.61)

No new syntax. Stdlib package `பரிமாற்றம்` (`கொணர் "பரிமாற்றம்"`):
server (`கோரிக்கை` / `பதிலளிப்பு` / `கையாளு` / `வழிப்படுத்தி`,
`எழுது` / `எழுதுசரம்` / `எழுதுதலைப்புகள்` / `எழுதுசரந்தலைப்புகள்`,
`சேவைஒன்று` / `கேட்டுசேவை` / `வழிசேவை`), and client (`பதில்`, `பெறு` /
`பதிவிடு` / `கோரு` / `கோருவிருப்பம்`). See `constructs/http.yaml`.

## SQL databases (Tamil-0.62)

No new syntax. Stdlib package `தரவுத்தளம்` (`கொணர் "தரவுத்தளம்"`) provides
a shared connection/prepared-statement API over an internal C backend vtable.
SQLite (`"sqlite"` / `"sqlite3"`) is the first backend. Typed bind and column
functions cover null, integer, float, UTF-8 text, and byte blobs. See
`constructs/database.yaml`.

## File I/O (Tamil-0.63)

No new syntax. Stdlib package `கோப்பு` (`கொணர் "கோப்பு"`): opaque `கை`
handles, `திற` / `உருவாக்கு` / `படி` / `எழுது` / `விடு`, plus whole-file
`படிஅனை` / `எழுதுஅனை`. See `constructs/file.yaml`.

## Time (Tamil-0.64)

No new syntax. Stdlib package `நேரம்` (`கொணர் "நேரம்"`): `காலம்` /
`தருணம்`, `இப்போ` / `உறங்கு` / `கழித்தது`, Unix helpers, RFC3339
`சரம்ஆக்கு`, and duration unit constructors. See `constructs/time.yaml`.

## Formatting (Tamil-0.65)

No new syntax. Stdlib package `வடிவம்` (`கொணர் "வடிவம்"`): `%s`/`%%`
`வடிவமை` over string args, scalar `*உரை` helpers, and `இணை`.
See `constructs/fmt.yaml`.

## Variadic parameters (Tamil-0.66 / 0.69)

Final parameter may be `பெயர் ...வகை` (Go-style). Inside the function the
parameter has type `[]வகை`. Calls pack zero or more trailing args into that
slice. Tamil-0.69 adds `slice...` on the final call argument to expand a
`[]T` into the variadic parameter. See `constructs/variadic.yaml`.
`வடிவம்.வடிவமை` uses `...சரம்`.

## Constants (Tamil-0.67 / 0.68)

`மாறிலி` declares compile-time constants at package or block scope.
RHS must be a constant expression; values are folded and inlined.
Tamil-0.68 adds parenthesized groups and the predeclared identifier `iota`
(0, 1, 2, … per spec in a group; omitted RHS/type repeat the previous spec).
See `constructs/const.yaml`. Keyword is **not** `நிலை` (bool type).

## Entry convention

Package-level `மாறிலி` (Tamil-0.67+) and `மாறி` (Tamil-0.70) may appear
before functions. Optional `வெளி` exports names to other packages.

1. `தொகுப்பு தொடக்கம்` (every file in the package)
2. `செயல்பாடு தொடக்கம்() { … }` (exactly once in the package)
