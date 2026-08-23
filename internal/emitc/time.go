package emitc

import (
	_ "embed"
	"strings"

	"niraluli/internal/ast"
)

//go:embed time_runtime.inc
var timeRuntimeC string

func (e *emitter) markTimeNeeds(pkgNames []string) {
	for _, n := range pkgNames {
		if n == "நேரம்" {
			e.needTime = true
			e.needArena = true
			return
		}
	}
}

func (e *emitter) writeTimeRuntime(b *strings.Builder) {
	if !e.needTime {
		return
	}
	e.needArena = true
	b.WriteString(timeRuntimeC)
	b.WriteByte('\n')
}

func (e *emitter) writeTimeIntrinsic(b *strings.Builder, fn *ast.FuncDecl) bool {
	if e.pkg != "நேரம்" || fn == nil || fn.Name == nil || fn.Recv != nil {
		return false
	}
	var call string
	switch fn.Name.Name {
	case "நானோவினாடி":
		call = "\treturn " + cIdent("n") + ";\n"
	case "நுண்ணியவினாடி":
		call = "\treturn " + cIdent("n") + " * 1000LL;\n"
	case "மில்லிவினாடி":
		call = "\treturn " + cIdent("n") + " * 1000000LL;\n"
	case "வினாடி":
		call = "\treturn " + cIdent("n") + " * 1000000000LL;\n"
	case "இப்போ":
		call = "\treturn uli_time_now_unix_nano();\n"
	case "உறங்கு":
		call = "\tuli_time_sleep(" + cIdent("க") + ");\n"
	case "கழித்தது":
		call = "\treturn uli_time_now_unix_nano() - " + cIdent("தொடக்கம்") + ";\n"
	case "யூனிக்ஸ்":
		call = "\treturn " + cIdent("த") + " / 1000000000LL;\n"
	case "யூனிக்ஸ்நானோ":
		call = "\treturn " + cIdent("த") + ";\n"
	case "சரம்ஆக்கு":
		call = "\treturn uli_time_format_rfc3339(" + cIdent("த") + ");\n"
	default:
		return false
	}
	e.needTime = true
	e.needArena = true
	b.WriteString(e.cFuncSig(fn))
	b.WriteString(" {\n")
	b.WriteString(call)
	b.WriteString("}\n")
	return true
}
