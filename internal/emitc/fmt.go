package emitc

import (
	_ "embed"
	"strings"

	"niraluli/internal/ast"
)

//go:embed fmt_runtime.inc
var fmtRuntimeC string

func (e *emitter) markFmtNeeds(pkgNames []string) {
	for _, n := range pkgNames {
		if n == "வடிவம்" {
			e.needFmt = true
			e.needArena = true
			e.needSlice = true
			return
		}
	}
}

func (e *emitter) writeFmtRuntime(b *strings.Builder) {
	if !e.needFmt {
		return
	}
	e.needArena = true
	e.needSlice = true
	b.WriteString(fmtRuntimeC)
	b.WriteByte('\n')
}

func (e *emitter) writeFmtIntrinsic(b *strings.Builder, fn *ast.FuncDecl) bool {
	if e.pkg != "வடிவம்" || fn == nil || fn.Name == nil || fn.Recv != nil {
		return false
	}
	var call string
	switch fn.Name.Name {
	case "எண்உரை":
		call = "\treturn uli_fmt_int(" + cIdent("n") + ");\n"
	case "மிதவைஉரை":
		call = "\treturn uli_fmt_float(" + cIdent("f") + ");\n"
	case "நிலைஉரை":
		call = "\treturn uli_fmt_bool(" + cIdent("b") + ");\n"
	case "வடிவமை":
		call = "\treturn uli_fmt_sprintf(" + cIdent("வார்ப்பு") + ", " + cIdent("மதிப்புகள்") + ".data, " + cIdent("மதிப்புகள்") + ".len);\n"
	case "இணை":
		call = "\treturn uli_fmt_join(" + cIdent("பகுதிகள்") + ".data, " + cIdent("பகுதிகள்") + ".len, " + cIdent("பிரிப்பான்") + ");\n"
	default:
		return false
	}
	e.needFmt = true
	e.needArena = true
	e.needSlice = true
	b.WriteString(e.cFuncSig(fn))
	b.WriteString(" {\n")
	b.WriteString(call)
	b.WriteString("}\n")
	return true
}
