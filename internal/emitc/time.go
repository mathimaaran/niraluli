package emitc

import (
	_ "embed"
	"strings"

	"niraluli/internal/ast"
)

//go:embed time_runtime.inc
var timeRuntimeC string

//go:embed time_timer_runtime.inc
var timeTimerRuntimeC string

func (e *emitter) markTimeNeeds(pkgNames []string) {
	for _, n := range pkgNames {
		if n == "நேரம்" {
			e.needTime = true
			e.needTimeTimers = true
			e.needChan = true
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

func (e *emitter) writeTimeTimerRuntime(b *strings.Builder) {
	if !e.needTimeTimers {
		return
	}
	e.needChan = true
	e.needTime = true
	e.needArena = true
	b.WriteString(timeTimerRuntimeC)
	b.WriteByte('\n')
}

func (e *emitter) writeTimeIntrinsic(b *strings.Builder, fn *ast.FuncDecl) bool {
	if e.pkg != "நேரம்" || fn == nil || fn.Name == nil || fn.Recv != nil {
		return false
	}
	errType := cPkgIdent("நேரம்", "பிழை") + " *"
	ret := e.retCName(e.resultTypes(fn))
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
	case "சரம்ஆக்குஇடம்":
		call = "\treturn uli_time_format_rfc3339_loc(" + cIdent("த") + ", " + cIdent("இடம்") + ");\n"
	case "பகு":
		call = "\tuli_time_parse_result r = uli_time_parse_rfc3339(" + cIdent("உரை") + ");\n" +
			"\treturn (" + ret + "){ r.value, (" + errType + ")r.err };\n"
	case "பின்னர்":
		e.needTimeTimers = true
		e.needChan = true
		call = "\treturn uli_time_after(" + cIdent("க") + ");\n"
	case "துடிப்பு":
		e.needTimeTimers = true
		e.needChan = true
		call = "\treturn uli_time_tick(" + cIdent("க") + ");\n"
	case "நிறுத்து":
		e.needTimeTimers = true
		e.needChan = true
		call = "\treturn uli_time_stop(" + cIdent("ச") + ");\n"
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
