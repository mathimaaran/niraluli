package emitc

import (
	_ "embed"
	"fmt"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/check"
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

// writeFmtSprintfCall emits வடிவம்.வடிவமை with typed verbs by stringifying args (Tamil-0.77).
func (e *emitter) writeFmtSprintfCall(b *strings.Builder, call *ast.CallExpr) {
	plan := e.info.FmtPlans[call]
	if plan == nil {
		return
	}
	e.needFmt = true
	e.needArena = true
	e.needSlice = true
	format := check.NormalizeFmtFormat(plan.Format)
	n := len(plan.Args)
	b.WriteString("({ ")
	if n > 0 {
		fmt.Fprintf(b, "const char *__uli_fmt_args[%d]; ", n)
		for i, arg := range plan.Args {
			fmt.Fprintf(b, "__uli_fmt_args[%d] = ", i)
			e.writeFmtArgString(b, arg)
			b.WriteString("; ")
		}
		fmt.Fprintf(b, "uli_fmt_sprintf(\"%s\", __uli_fmt_args, %dLL);", escapeCString(format), n)
	} else {
		fmt.Fprintf(b, "uli_fmt_sprintf(\"%s\", NULL, 0LL);", escapeCString(format))
	}
	b.WriteString(" })")
}

func (e *emitter) writeFmtArgString(b *strings.Builder, arg ast.Expr) {
	t := e.typeOf(arg)
	if e.info != nil {
		if under, ok := e.info.Underlying[t]; ok {
			t = under
		}
	}
	switch {
	case t == check.TypeString:
		e.writeExpr(b, arg)
	case t == check.TypeBool:
		b.WriteString("uli_fmt_bool(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case t == check.TypeFloat:
		b.WriteString("uli_fmt_float(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	default:
		b.WriteString("uli_fmt_int((int64_t)(")
		e.writeExpr(b, arg)
		b.WriteString("))")
	}
}
