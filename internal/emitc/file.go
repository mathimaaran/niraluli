package emitc

import (
	_ "embed"
	"strings"

	"niraluli/internal/ast"
)

//go:embed file_runtime.inc
var fileRuntimeC string

func (e *emitter) markFileNeeds(pkgNames []string) {
	for _, n := range pkgNames {
		if n == "கோப்பு" {
			e.needFile = true
			e.needArena = true
			e.needSlice = true
			return
		}
	}
}

func (e *emitter) writeFileRuntime(b *strings.Builder) {
	if !e.needFile {
		return
	}
	e.needArena = true
	e.needSlice = true
	b.WriteString(fileRuntimeC)
	b.WriteByte('\n')
}

func (e *emitter) writeFileIntrinsic(b *strings.Builder, fn *ast.FuncDecl) bool {
	if e.pkg != "கோப்பு" || fn == nil || fn.Name == nil || fn.Recv != nil {
		return false
	}
	errType := cPkgIdent("கோப்பு", "பிழை") + " *"
	ret := e.retCName(e.resultTypes(fn))
	var call string
	switch fn.Name.Name {
	case "திற":
		call = "\tuli_file_i64_result r = uli_file_open_read(" + cIdent("பாதை") + ");\n\treturn (" + ret + "){ r.value, (" + errType + ")r.err };\n"
	case "உருவாக்கு":
		call = "\tuli_file_i64_result r = uli_file_create(" + cIdent("பாதை") + ");\n\treturn (" + ret + "){ r.value, (" + errType + ")r.err };\n"
	case "படி":
		call = "\tuli_file_i64_result r = uli_file_read(" + cIdent("க") + ", " + cIdent("இடம்") + ".data, " + cIdent("இடம்") + ".len);\n\treturn (" + ret + "){ r.value, (" + errType + ")r.err };\n"
	case "எழுது":
		call = "\tuli_file_i64_result r = uli_file_write(" + cIdent("க") + ", " + cIdent("தரவு") + ".data, " + cIdent("தரவு") + ".len);\n\treturn (" + ret + "){ r.value, (" + errType + ")r.err };\n"
	case "விடு":
		call = "\treturn (" + errType + ")uli_file_close(" + cIdent("க") + ");\n"
	case "படிஅனை":
		call = "\tuli_file_blob_result r = uli_file_read_all(" + cIdent("பாதை") + ");\n" +
			"\tuli_slice_u8 value = { r.data, r.len, r.len };\n" +
			"\treturn (" + ret + "){ value, (" + errType + ")r.err };\n"
	case "எழுதுஅனை":
		call = "\treturn (" + errType + ")uli_file_write_all(" + cIdent("பாதை") + ", " + cIdent("தரவு") + ".data, " + cIdent("தரவு") + ".len);\n"
	default:
		return false
	}
	e.needFile = true
	e.needArena = true
	e.needSlice = true
	b.WriteString(e.cFuncSig(fn))
	b.WriteString(" {\n")
	b.WriteString(call)
	b.WriteString("}\n")
	return true
}
