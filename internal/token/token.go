// Package token defines Niraluli Tamil-0 lexical tokens.
package token

// Kind is the type of a token.
type Kind int

const (
	ILLEGAL Kind = iota
	EOF
	COMMENT // retained only if we ever expose comments; lexer skips them

	IDENT
	INT
	FLOAT
	STRING

	// Keywords / reserved words (Tamil-0 frozen)
	PACKAGE    // தொகுப்பு
	FUNC       // செயல்பாடு
	VAR        // மாறி
	CONST      // மாறிலி (Tamil-0.67)
	IF         // எனில்
	ELSE       // இல்லையேல்
	RETURN     // திருப்பு
	TYPE_INT   // முழுஎண்
	TYPE_BOOL  // நிலை
	TYPE_FLOAT // மிதவைஎண்
	TYPE_BYTE  // இருமி8 (byte / uint8)
	TYPE_RUNE  // இருமி32 (rune / int32)
	TRUE       // மெய்
	FALSE      // பொய்
	PRINT      // பதிப்பி (builtin name; reserved)
	FOR        // சுழல்
	BREAK      // முறி
	CONTINUE    // தொடர்
	TYPE_STRING // சரம்
	LEN         // நீளம் (builtin; reserved)
	RANGE       // ஒவ்வொரு
	APPEND      // சேர் (builtin; reserved)
	MAKE        // ஆக்கு (builtin; reserved)
	COPY        // நகல் (builtin; reserved)
	CAP         // திறன் (builtin; reserved)
	TYPE        // வகை
	STRUCT      // அமைப்பு
	IMPORT      // கொணர்
	EXPORT      // வெளி
	NIL         // இன்மை
	SWITCH      // திசைவி
	DEFAULT     // மற்றபடி
	MAP         // அகராதி (map type)
	DELETE      // நீக்கு (builtin; reserved)
	DEFER       // தள்ளிவை
	GO          // இழை
	CHAN        // தடம்
	SELECT      // தடத்தேர்வு
	CLOSE   // மூடு (builtin; reserved)
	PANIC   // அலறு (builtin; reserved)
	RECOVER // மீள் (builtin; reserved)

	ADD // +
	SUB // -
	MUL // * (also unary deref / pointer type)
	QUO // /
	REM // %
	AND // & (unary address-of)
	OR  // | (constraint union in type params)
	ARROW // <-

	EQL    // ==
	NEQ    // !=
	LSS    // <
	GTR    // >
	LEQ    // <=
	GEQ    // >=
	ASSIGN // =
	DEFINE // :=
	NOT    // !
	COLON  // :
	SEMICOLON // ;

	LPAREN // (
	RPAREN // )
	LBRACK // [
	RBRACK // ]
	LBRACE // {
	RBRACE // }
	COMMA  // ,
	PERIOD // .
	ELLIPSIS // ...
)

var kindNames = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",
	IDENT:   "IDENT",
	INT:     "INT",
	FLOAT:   "FLOAT",
	STRING:  "STRING",

	PACKAGE:   "PACKAGE",
	FUNC:      "FUNC",
	VAR:       "VAR",
	CONST:     "CONST",
	IF:        "IF",
	ELSE:      "ELSE",
	RETURN:    "RETURN",
	TYPE_INT:  "TYPE_INT",
	TYPE_BOOL: "TYPE_BOOL",
	TYPE_FLOAT: "TYPE_FLOAT",
	TYPE_BYTE:  "TYPE_BYTE",
	TYPE_RUNE:  "TYPE_RUNE",
	TRUE:      "TRUE",
	FALSE:     "FALSE",
	PRINT:     "PRINT",
	FOR:       "FOR",
	BREAK:     "BREAK",
	CONTINUE:    "CONTINUE",
	TYPE_STRING: "TYPE_STRING",
	LEN:         "LEN",
	RANGE:       "RANGE",
	APPEND:      "APPEND",
	MAKE:        "MAKE",
	COPY:        "COPY",
	CAP:         "CAP",
	TYPE:        "TYPE",
	STRUCT:      "STRUCT",
	IMPORT:      "IMPORT",
	EXPORT:      "EXPORT",
	NIL:         "NIL",
	SWITCH:      "SWITCH",
	DEFAULT:     "DEFAULT",
	MAP:         "MAP",
	DELETE:      "DELETE",
	DEFER:       "DEFER",
	GO:          "GO",
	CHAN:        "CHAN",
	SELECT:      "SELECT",
	CLOSE:   "CLOSE",
	PANIC:   "PANIC",
	RECOVER: "RECOVER",

	ADD: "ADD",
	SUB: "SUB",
	MUL: "MUL",
	QUO: "QUO",
	REM: "REM",
	AND: "AND",
	OR:  "OR",
	ARROW: "ARROW",

	EQL:    "EQL",
	NEQ:    "NEQ",
	LSS:    "LSS",
	GTR:    "GTR",
	LEQ:    "LEQ",
	GEQ:    "GEQ",
	ASSIGN: "ASSIGN",
	DEFINE: "DEFINE",
	NOT:    "NOT",
	COLON:  "COLON",
	SEMICOLON: "SEMICOLON",

	LPAREN: "LPAREN",
	RPAREN: "RPAREN",
	LBRACK: "LBRACK",
	RBRACK: "RBRACK",
	LBRACE: "LBRACE",
	RBRACE: "RBRACE",
	COMMA:  "COMMA",
	PERIOD: "PERIOD",
	ELLIPSIS: "ELLIPSIS",
}

func (k Kind) String() string {
	if int(k) >= 0 && int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "Kind(?)"
}

// Keywords maps Tamil-0 reserved words to token kinds.
var Keywords = map[string]Kind{
	"தொகுப்பு":  PACKAGE,
	"செயல்பாடு": FUNC,
	"மாறி":      VAR,
	"மாறிலி":    CONST,
	"எனில்":     IF,
	"இல்லையேல்": ELSE,
	"திருப்பு":  RETURN,
	"முழுஎண்":   TYPE_INT,
	"நிலை":      TYPE_BOOL,
	"மிதவைஎண்":  TYPE_FLOAT,
	"இருமி8":    TYPE_BYTE,
	"இருமி32":   TYPE_RUNE,
	"மெய்":      TRUE,
	"பொய்":      FALSE,
	"பதிப்பி":   PRINT,
	"சுழல்":     FOR,
	"முறி":      BREAK,
	"தொடர்":     CONTINUE,
	"சரம்":      TYPE_STRING,
	"நீளம்":     LEN,
	"ஒவ்வொரு":   RANGE,
	"சேர்":      APPEND,
	"ஆக்கு":    MAKE,
	"நகல்":     COPY,
	"திறன்":    CAP,
	"வகை":      TYPE,
	"அமைப்பு":   STRUCT,
	"கொணர்":    IMPORT,
	"வெளி":     EXPORT,
	"இன்மை":    NIL,
	"திசைவி":   SWITCH,
	"மற்றபடி":  DEFAULT,
	"அகராதி":   MAP,
	"நீக்கு":   DELETE,
	"தள்ளிவை": DEFER,
	"இழை":      GO,
	"தடம்":     CHAN,
	"தடத்தேர்வு": SELECT,
	"மூடு": CLOSE,
	"அலறு": PANIC,
	"மீள்":  RECOVER,
}

// LookupIdent returns the keyword kind or IDENT.
func LookupIdent(name string) Kind {
	if k, ok := Keywords[name]; ok {
		return k
	}
	return IDENT
}

// Pos is a 1-based line/column position in the source (column in runes).
type Pos struct {
	Line, Col int
	Offset    int // byte offset
}

// Token is a single lexeme.
type Token struct {
	Kind Kind
	Lit  string
	Pos  Pos
}
