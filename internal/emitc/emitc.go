// Package emitc emits Niraluli programs as C99 source.
package emitc

import (
	"fmt"
	"strconv"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/check"
	"niraluli/internal/token"
)

// Emit returns complete C source for a single type-checked package file.
func Emit(f *ast.File, info *check.Info) (string, error) {
	pkg := "தொடக்கம்"
	if f != nil && f.Package != nil && f.Package.Name != nil {
		pkg = f.Package.Name.Name
	}
	return EmitProgram(&check.ProgramInfo{
		Entry: pkg,
		Pkgs:  []*check.PkgInfo{{Name: pkg, File: f}},
		Info:  info,
	})
}

// EmitProgram returns C source for a multi-package program.
func EmitProgram(pi *check.ProgramInfo) (string, error) {
	info := pi.Info
	if info == nil {
		info = &check.Info{
			Types:      map[ast.Expr]check.Type{},
			Structs:    map[check.Type]*check.StructInfo{},
			Defined:    map[check.Type]*check.DefinedInfo{},
			Underlying: map[check.Type]check.Type{},
			TypeByName: map[string]check.Type{},
			PtrElem:    map[check.Type]check.Type{},
		}
	}
	if info.Structs == nil {
		info.Structs = map[check.Type]*check.StructInfo{}
	}
	if info.Defined == nil {
		info.Defined = map[check.Type]*check.DefinedInfo{}
	}
	if info.Underlying == nil {
		info.Underlying = map[check.Type]check.Type{}
	}
	if info.TypeByName == nil {
		info.TypeByName = map[string]check.Type{}
	}
	if info.PtrElem == nil {
		info.PtrElem = map[check.Type]check.Type{}
	}
	if info.SliceElem == nil {
		info.SliceElem = map[check.Type]check.Type{}
	}
	if info.TupleElems == nil {
		info.TupleElems = map[check.Type][]check.Type{}
	}
	if info.Arrays == nil {
		info.Arrays = map[check.Type]check.ArrayInfo{}
	}
	if info.Maps == nil {
		info.Maps = map[check.Type]check.MapInfo{}
	}
	if info.CallInst == nil {
		info.CallInst = map[*ast.CallExpr]*check.MonoInst{}
	}
	if info.GenericMethodTemplates == nil {
		info.GenericMethodTemplates = map[*ast.FuncDecl]bool{}
	}
	entry := pi.Entry
	if entry == "" {
		entry = "தொடக்கம்"
	}
	e := &emitter{info: info, entry: entry}
	return e.emitProgram(pi.Pkgs)
}

type emitter struct {
	info          *check.Info
	pkg           string // current package while emitting funcs
	entry         string
	importLocal   map[string]string // local qualifier → real package
	curFn         *ast.FuncDecl     // function being emitted (for naked return)
	fnHasDefer    bool              // current function contains தள்ளிவை
	typeSubst     map[string]check.Type
	monoName      string // override C name when emitting a monomorphized function
	needConcat    bool
	needSlice     bool
	needAppend    bool
	needMake      bool
	needCopy      bool
	needMap       bool
	needDefer     bool
	needFunc      bool // function values (Tamil-0.44)
	needChan      bool // channels (Tamil-0.48)
	needGo        bool // இழை (Tamil-0.48)
	needPanic     bool // அலறு / மீள் (Tamil-0.49)
	needNet       bool // வலை TCP sockets (Tamil-0.53)
	needHttp      bool // பரிமாற்றம் HTTP (Tamil-0.54)
	needDB        bool // தரவுத்தளம் SQL database API (Tamil-0.62)
	needFile      bool // கோப்பு file I/O (Tamil-0.63)
	needTime      bool // நேரம் wall clock + duration (Tamil-0.64)
	needFmt       bool // வடிவம் fmt-style formatting (Tamil-0.65)
	needPkgVarInit bool // package-level மாறி with runtime initializers
	needArena     bool
	needUTF8      bool
	needRuneStr   bool // சரம்(rune) conversion helper
	needStrBytes  bool // சரம் ↔ []இருமி8 / []இருமி32 helpers
	swID          int  // unique temps for திசைவி
	deferID       int  // unique deferred-call thunk ids
	goID          int  // unique temps for இழை / select / send
	deferThunks   strings.Builder
	funcTramps    strings.Builder
	funcTrampDone map[string]bool
	funcCallDone  map[string]bool
	promoted      map[string]check.Type // arena-promoted locals in current frame
	loopIter      map[string]string     // Tamil-0.46: source name → __it_* in for cond/post
	structsDone   bool                  // full struct bodies already emitted (before []Struct helpers)
	tupleDecls    [][]check.Type        // declared multi-result function shapes
}

func (e *emitter) emitProgram(pkgs []*check.PkgInfo) (string, error) {
	e.markSliceNeedsFromStructs()
	e.markPanicNeeds(pkgs)
	for _, p := range pkgs {
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		for _, d := range p.File.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				if results := e.resultTypes(fn); len(results) > 1 {
					e.tupleDecls = append(e.tupleDecls, results)
				}
			}
		}
	}
	var pkgNames []string
	for _, p := range pkgs {
		pkgNames = append(pkgNames, p.Name)
	}
	e.markNetNeeds(pkgNames)
	e.markHttpNeeds(pkgNames)
	e.markDBNeeds(pkgNames)
	e.markFileNeeds(pkgNames)
	e.markTimeNeeds(pkgNames)
	e.markFmtNeeds(pkgNames)

	var body strings.Builder
	for _, p := range pkgs {
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		for _, d := range p.File.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				if len(fn.TypeParams) > 0 {
					continue
				}
				if e.info != nil && e.info.GenericMethodTemplates[fn] {
					continue
				}
				if fn.Name != nil && fn.Name.Name == "தொடக்கம்" && fn.Recv == nil {
					continue
				}
				e.writeFunc(&body, fn)
				body.WriteString("\n")
			}
		}
	}
	if e.info != nil {
		for _, inst := range e.info.Instantiations {
			e.pkg = inst.Pkg
			e.writeMonoFunc(&body, inst)
			body.WriteString("\n")
		}
		for _, t := range sortedStructTypes(e.info) {
			si := e.info.Structs[t]
			if si.Schematic {
				continue
			}
			for _, mi := range sortedStructMethods(si) {
				if mi.Decl == nil || !e.info.GenericMethodTemplates[mi.Decl] {
					continue
				}
				e.pkg = si.Pkg
				e.writeMonoMethod(&body, si, mi)
				body.WriteString("\n")
			}
		}
	}
	for _, p := range pkgs {
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		for _, d := range p.File.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				if fn.Name != nil && fn.Name.Name == "தொடக்கம்" && fn.Recv == nil {
					e.writeFunc(&body, fn)
					body.WriteString("\n")
				}
			}
		}
	}

	if e.info != nil && len(e.info.Maps) > 0 {
		e.needMap = true
	}
	if e.info != nil && len(e.info.Funcs) > 0 {
		e.needFunc = true
	}
	e.markChanNeeds()
	if e.needArena || e.needConcat || e.needAppend || e.needSlice || e.needMake || e.needRuneStr || e.needStrBytes || e.needMap || e.needDefer || e.needFunc || e.needChan || e.needGo || e.needNet || e.needHttp || e.needDB || e.needFile || e.needTime || e.needFmt {
		e.needArena = true
	}

	var b strings.Builder
	if e.needArena {
		b.WriteString("#define _GNU_SOURCE 1\n")
	}
	b.WriteString("/* Generated by uli — do not edit by hand */\n")
	b.WriteString("#include <stdio.h>\n")
	b.WriteString("#include <stdint.h>\n")
	b.WriteString("#include <stdlib.h>\n")
	b.WriteString("#include <string.h>\n")
	if e.needPanic {
		b.WriteString("#include <setjmp.h>\n")
	}
	b.WriteString("\n")
	b.WriteString("static void uli_print_int(int64_t v) { printf(\"%lld\\n\", (long long)v); }\n")
	b.WriteString("static void uli_print_float(double v) { printf(\"%g\\n\", v); }\n")
	b.WriteString("static void uli_print_str(const char *s) { printf(\"%s\\n\", s); }\n")
	b.WriteString("static void uli_print_bool(int v) { printf(\"%s\\n\", v ? \"மெய்\" : \"பொய்\"); }\n")
	b.WriteString("static void uli_print_int_n(int64_t v) { printf(\"%lld\", (long long)v); }\n")
	b.WriteString("static void uli_print_float_n(double v) { printf(\"%g\", v); }\n")
	b.WriteString("static void uli_print_str_n(const char *s) { printf(\"%s\", s ? s : \"\"); }\n")
	b.WriteString("static void uli_print_bool_n(int v) { printf(\"%s\", v ? \"மெய்\" : \"பொய்\"); }\n")
	b.WriteString("static void uli_print_quoted_n(const char *s) {\n")
	b.WriteString("\tprintf(\"\\\"\");\n")
	b.WriteString("\tfor (const unsigned char *p = (const unsigned char *)(s ? s : \"\"); *p; p++) {\n")
	b.WriteString("\t\tif (*p == '\"' || *p == '\\\\') printf(\"\\\\%c\", *p);\n")
	b.WriteString("\t\telse printf(\"%c\", *p);\n")
	b.WriteString("\t}\n")
	b.WriteString("\tprintf(\"\\\"\");\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_print_ptr_n(const void *p) {\n")
	b.WriteString("\tif (!p) printf(\"இன்மை\");\n")
	b.WriteString("\telse printf(\"%p\", p);\n")
	b.WriteString("}\n")

	e.writeFuncTypedef(&b)
	e.writeGCRuntime(&b)
	e.writeNetRuntime(&b)
	e.writeDBRuntime(&b)
	e.writeFileRuntime(&b)
	e.writeTimeRuntime(&b)
	e.writeFmtRuntime(&b)
	e.writeChanRuntime(&b)
	e.writePanicRuntime(&b)
	if e.needConcat {
		e.needArena = true
		b.WriteString("static const char *uli_concat_many(size_t n, const char *const *parts) {\n")
		b.WriteString("\tsize_t total = 0, nonempty = 0;\n")
		b.WriteString("\tconst char *single = \"\";\n")
		b.WriteString("\tfor (size_t i = 0; i < n; i++) {\n")
		b.WriteString("\t\tconst char *p = parts[i] ? parts[i] : \"\";\n")
		b.WriteString("\t\tsize_t len = strlen(p);\n")
		b.WriteString("\t\tif (len > SIZE_MAX - total - 1) abort();\n")
		b.WriteString("\t\ttotal += len;\n")
		b.WriteString("\t\tif (len) { nonempty++; single = p; }\n")
		b.WriteString("\t}\n")
		b.WriteString("\tif (nonempty == 0) return \"\";\n")
		b.WriteString("\tif (nonempty == 1) return single;\n")
		b.WriteString("\tchar *r = (char *)uli_arena_alloc(total + 1);\n")
		b.WriteString("\tsize_t off = 0;\n")
		b.WriteString("\tfor (size_t i = 0; i < n; i++) {\n")
		b.WriteString("\t\tconst char *p = parts[i] ? parts[i] : \"\";\n")
		b.WriteString("\t\tsize_t len = strlen(p);\n")
		b.WriteString("\t\tmemcpy(r + off, p, len); off += len;\n")
		b.WriteString("\t}\n")
		b.WriteString("\tr[off] = '\\0';\n")
		b.WriteString("\treturn r;\n")
		b.WriteString("}\n")
	}
	if e.needStrBytes {
		e.needUTF8 = true
		e.needRuneStr = true
		e.needSlice = true
		e.needArena = true
	}
	// Map handles are pointers to opaque table structs, so their forwards must
	// precede any named struct body that contains a map field.
	e.writeMapForwards(&b)
	if e.needSlice || e.needAppend || e.needMake || e.needCopy {
		e.needSlice = true
		b.WriteString("typedef struct { int64_t *data; int64_t len; int64_t cap; } uli_slice_i64;\n")
		b.WriteString("typedef struct { int *data; int64_t len; int64_t cap; } uli_slice_bool;\n")
		b.WriteString("typedef struct { const char **data; int64_t len; int64_t cap; } uli_slice_str;\n")
		b.WriteString("typedef struct { double *data; int64_t len; int64_t cap; } uli_slice_f64;\n")
		b.WriteString("typedef struct { uint8_t *data; int64_t len; int64_t cap; } uli_slice_u8;\n")
		b.WriteString("typedef struct { int32_t *data; int64_t len; int64_t cap; } uli_slice_i32;\n")
		b.WriteString("static int64_t uli_slice_get_i64(uli_slice_i64 s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_i64(uli_slice_i64 s, int64_t i, int64_t v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static int uli_slice_get_bool(uli_slice_bool s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_bool(uli_slice_bool s, int64_t i, int v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static const char *uli_slice_get_str(uli_slice_str s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_str(uli_slice_str s, int64_t i, const char *v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static double uli_slice_get_f64(uli_slice_f64 s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_f64(uli_slice_f64 s, int64_t i, double v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static uint8_t uli_slice_get_u8(uli_slice_u8 s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_u8(uli_slice_u8 s, int64_t i, uint8_t v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static int32_t uli_slice_get_i32(uli_slice_i32 s, int64_t i) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
		b.WriteString("static void uli_slice_set_i32(uli_slice_i32 s, int64_t i, int32_t v) {\n")
		b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
		b.WriteString("static uli_slice_i64 uli_sub_i64(uli_slice_i64 s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_i64){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_bool uli_sub_bool(uli_slice_bool s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_bool){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_str uli_sub_str(uli_slice_str s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_str){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_f64 uli_sub_f64(uli_slice_f64 s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_f64){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_u8 uli_sub_u8(uli_slice_u8 s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_u8){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_i32 uli_sub_i32(uli_slice_i32 s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
		b.WriteString("\treturn (uli_slice_i32){ s.data + lo, hi - lo, s.cap - lo };\n}\n")
		b.WriteString("static uli_slice_i64 uli_sub3_i64(uli_slice_i64 s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_i64){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static uli_slice_bool uli_sub3_bool(uli_slice_bool s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_bool){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static uli_slice_str uli_sub3_str(uli_slice_str s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_str){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static uli_slice_f64 uli_sub3_f64(uli_slice_f64 s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_f64){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static uli_slice_u8 uli_sub3_u8(uli_slice_u8 s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_u8){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static uli_slice_i32 uli_sub3_i32(uli_slice_i32 s, int64_t lo, int64_t hi, int64_t max) {\n")
		b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
		b.WriteString("\treturn (uli_slice_i32){ s.data + lo, hi - lo, max - lo };\n}\n")
		b.WriteString("static const char *uli_sub_string(const char *s, int64_t lo, int64_t hi) {\n")
		b.WriteString("\tsize_t n = strlen(s);\n")
		b.WriteString("\tif (lo < 0 || hi < lo || (size_t)hi > n) abort();\n")
		b.WriteString("\tchar *r = (char *)uli_arena_alloc((size_t)(hi - lo) + 1);\n")
		b.WriteString("\tmemcpy(r, s + lo, (size_t)(hi - lo));\n")
		b.WriteString("\tr[hi - lo] = '\\0';\n\treturn r;\n}\n")
		// Struct forwards + bodies interleaved with nested []T so []Struct
		// helpers see complete element types (pointers only need forwards).
		e.writeStructForwards(&b)
		e.writeStructsAndNestedSlices(&b)
	}
	e.writeArrayTypedefs(&b)
	// Struct bodies may be map values; emit before map entry layouts.
	if e.needMap && e.info != nil && len(e.info.Structs) > 0 && !e.structsDone {
		e.writeStructForwards(&b)
		for _, t := range topoStructTypes(e.info) {
			e.writeStructBody(&b, t)
		}
		b.WriteByte('\n')
		e.structsDone = true
	}
	// Tuple return structs may reference slice/array/map typedefs.
	e.writeTupleTypedefs(&b)
	// Map funcs (hash/eq helpers) are emitted after struct eq below.
	if e.needAppend {
		e.needArena = true
		b.WriteString("static uli_slice_i64 uli_append_i64(uli_slice_i64 s, int64_t v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tint64_t *nd = (int64_t *)uli_arena_alloc((size_t)ncap * sizeof(int64_t));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(int64_t));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		b.WriteString("static uli_slice_bool uli_append_bool(uli_slice_bool s, int v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tint *nd = (int *)uli_arena_alloc((size_t)ncap * sizeof(int));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(int));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		b.WriteString("static uli_slice_str uli_append_str(uli_slice_str s, const char *v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tconst char **nd = (const char **)uli_arena_alloc((size_t)ncap * sizeof(const char *));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(const char *));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		b.WriteString("static uli_slice_f64 uli_append_f64(uli_slice_f64 s, double v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tdouble *nd = (double *)uli_arena_alloc((size_t)ncap * sizeof(double));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(double));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		b.WriteString("static uli_slice_u8 uli_append_u8(uli_slice_u8 s, uint8_t v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tuint8_t *nd = (uint8_t *)uli_arena_alloc((size_t)ncap * sizeof(uint8_t));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(uint8_t));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		b.WriteString("static uli_slice_i32 uli_append_i32(uli_slice_i32 s, int32_t v) {\n")
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		b.WriteString("\t\tint32_t *nd = (int32_t *)uli_arena_alloc((size_t)ncap * sizeof(int32_t));\n")
		b.WriteString("\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(int32_t));\n")
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
		e.writeNestedSliceAppend(&b)
	}
	if e.needMake {
		e.needArena = true
		e.needSlice = true
		e.writeMakeRuntime(&b)
	}
	if e.needCopy {
		e.needSlice = true
		e.writeCopyRuntime(&b)
	}
	if e.needUTF8 {
		b.WriteString("static int64_t uli_utf8_next(const char *s, int64_t *ip) {\n")
		b.WriteString("\tint64_t i = *ip;\n")
		b.WriteString("\tunsigned char c = (unsigned char)s[i];\n")
		b.WriteString("\tif (c == 0) return 0;\n")
		b.WriteString("\tif (c < 0x80) { *ip = i + 1; return c; }\n")
		b.WriteString("\tif ((c & 0xE0) == 0xC0) {\n")
		b.WriteString("\t\tint64_t r = ((c & 0x1F) << 6) | ((unsigned char)s[i+1] & 0x3F);\n")
		b.WriteString("\t\t*ip = i + 2; return r;\n")
		b.WriteString("\t}\n")
		b.WriteString("\tif ((c & 0xF0) == 0xE0) {\n")
		b.WriteString("\t\tint64_t r = ((c & 0x0F) << 12) | (((unsigned char)s[i+1] & 0x3F) << 6) | ((unsigned char)s[i+2] & 0x3F);\n")
		b.WriteString("\t\t*ip = i + 3; return r;\n")
		b.WriteString("\t}\n")
		b.WriteString("\tint64_t r = ((c & 0x07) << 18) | (((unsigned char)s[i+1] & 0x3F) << 12) | (((unsigned char)s[i+2] & 0x3F) << 6) | ((unsigned char)s[i+3] & 0x3F);\n")
		b.WriteString("\t*ip = i + 4; return r;\n")
		b.WriteString("}\n")
	}
	if e.needRuneStr {
		e.needArena = true
		b.WriteString("static const char *uli_rune_str(int64_t r) {\n")
		b.WriteString("\tunsigned char tmp[5];\n")
		b.WriteString("\tint n = 0;\n")
		b.WriteString("\tuint32_t u = (uint32_t)r;\n")
		b.WriteString("\tif (u < 0x80) { tmp[n++] = (unsigned char)u; }\n")
		b.WriteString("\telse if (u < 0x800) {\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0xC0 | (u >> 6));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | (u & 0x3F));\n")
		b.WriteString("\t} else if (u < 0x10000) {\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0xE0 | (u >> 12));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | ((u >> 6) & 0x3F));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | (u & 0x3F));\n")
		b.WriteString("\t} else {\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0xF0 | (u >> 18));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | ((u >> 12) & 0x3F));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | ((u >> 6) & 0x3F));\n")
		b.WriteString("\t\ttmp[n++] = (unsigned char)(0x80 | (u & 0x3F));\n")
		b.WriteString("\t}\n")
		b.WriteString("\ttmp[n] = 0;\n")
		b.WriteString("\tchar *out = (char *)uli_arena_alloc((size_t)n + 1);\n")
		b.WriteString("\tmemcpy(out, tmp, (size_t)n + 1);\n")
		b.WriteString("\treturn out;\n")
		b.WriteString("}\n")
	}
	if e.needStrBytes {
		b.WriteString("static const char *uli_bytes_str(uli_slice_u8 s) {\n")
		b.WriteString("\tchar *r = (char *)uli_arena_alloc((size_t)s.len + 1);\n")
		b.WriteString("\tif (s.len && s.data) memcpy(r, s.data, (size_t)s.len);\n")
		b.WriteString("\tr[s.len] = '\\0';\n\treturn r;\n}\n")
		b.WriteString("static const char *uli_runes_str(uli_slice_i32 s) {\n")
		b.WriteString("\tsize_t cap = (size_t)s.len * 4 + 1;\n")
		b.WriteString("\tchar *r = (char *)uli_arena_alloc(cap);\n")
		b.WriteString("\tsize_t n = 0;\n")
		b.WriteString("\tfor (int64_t i = 0; i < s.len; i++) {\n")
		b.WriteString("\t\tconst char *p = uli_rune_str((int64_t)s.data[i]);\n")
		b.WriteString("\t\tsize_t pn = strlen(p);\n")
		b.WriteString("\t\tmemcpy(r + n, p, pn);\n")
		b.WriteString("\t\tn += pn;\n")
		b.WriteString("\t}\n")
		b.WriteString("\tr[n] = '\\0';\n\treturn r;\n}\n")
		b.WriteString("static uli_slice_u8 uli_str_bytes(const char *s) {\n")
		b.WriteString("\tsize_t n = s ? strlen(s) : 0;\n")
		b.WriteString("\tif (n == 0) return (uli_slice_u8){0};\n")
		b.WriteString("\tuint8_t *d = (uint8_t *)uli_arena_alloc(n);\n")
		b.WriteString("\tmemcpy(d, s, n);\n")
		b.WriteString("\treturn (uli_slice_u8){ d, (int64_t)n, (int64_t)n };\n}\n")
		b.WriteString("static uli_slice_i32 uli_str_runes(const char *s) {\n")
		b.WriteString("\tif (!s || !*s) return (uli_slice_i32){0};\n")
		b.WriteString("\tint64_t n = 0, i = 0;\n")
		b.WriteString("\twhile (s[i]) { uli_utf8_next(s, &i); n++; }\n")
		b.WriteString("\tint32_t *d = (int32_t *)uli_arena_alloc((size_t)n * sizeof(int32_t));\n")
		b.WriteString("\ti = 0;\n")
		b.WriteString("\tfor (int64_t k = 0; k < n; k++) d[k] = (int32_t)uli_utf8_next(s, &i);\n")
		b.WriteString("\treturn (uli_slice_i32){ d, n, n };\n}\n")
	}
	e.writeHttpRuntime(&b)
	b.WriteByte('\n')

	// Named structs: bodies (if not already for []Struct), then eq/print.
	if e.info != nil && len(e.info.Structs) > 0 {
		if !e.structsDone {
			if !e.needSlice && !e.needAppend && !e.needMake && !e.needCopy {
				e.writeStructForwards(&b)
			}
			for _, t := range topoStructTypes(e.info) {
				if e.info.Structs[t].Schematic {
					continue
				}
				e.writeStructBody(&b, t)
			}
			b.WriteByte('\n')
			e.structsDone = true
		}
		for _, t := range topoStructTypes(e.info) {
			if e.info.Structs[t].Schematic {
				continue
			}
			if e.structComparable(t) {
				e.writeStructEqFn(&b, t)
			}
		}
		for _, t := range topoStructTypes(e.info) {
			if e.info.Structs[t].Schematic {
				continue
			}
			e.writeStructPrintFn(&b, t)
		}
	}
	e.writeArrayEqFuncs(&b)
	e.writeMapFuncs(&b)
	e.writeHttpMapBridge(&b)

	if e.needDefer {
		b.WriteString("typedef struct uli_defer_frame {\n")
		b.WriteString("\tvoid (*fn)(void *);\n")
		b.WriteString("\tvoid *arg;\n")
		b.WriteString("\tstruct uli_defer_frame *next;\n")
		b.WriteString("} uli_defer_frame;\n\n")
	}

	for _, p := range pkgs {
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		for _, d := range p.File.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				if len(fn.TypeParams) > 0 {
					continue
				}
				if e.info != nil && e.info.GenericMethodTemplates[fn] {
					continue
				}
				b.WriteString(e.cFuncSig(fn))
				b.WriteString(";\n")
			}
		}
	}
	if e.info != nil {
		for _, inst := range e.info.Instantiations {
			e.pkg = inst.Pkg
			b.WriteString(e.cMonoFuncSig(inst))
			b.WriteString(";\n")
		}
		for _, t := range sortedStructTypes(e.info) {
			si := e.info.Structs[t]
			if si.Schematic {
				continue
			}
			for _, mi := range sortedStructMethods(si) {
				if mi.Decl == nil || !e.info.GenericMethodTemplates[mi.Decl] {
					continue
				}
				e.pkg = si.Pkg
				prevSubst := e.typeSubst
				prevName := e.monoName
				e.typeSubst = map[string]check.Type{}
				for i, name := range si.GenericParamNames {
					if i < len(si.GenericTypeArgs) {
						e.typeSubst[name] = si.GenericTypeArgs[i]
					}
				}
				e.monoName = cPkgIdent(si.Pkg, si.Name+"_"+mi.Name)
				b.WriteString(e.cFuncSig(mi.Decl))
				b.WriteString(";\n")
				e.typeSubst = prevSubst
				e.monoName = prevName
			}
		}
	}
	b.WriteByte('\n')
	e.writePkgVarGlobals(&b, pkgs)
	if e.needPkgVarInit {
		e.writePkgVarInitFuncs(&b, pkgs)
	}
	if e.needDefer {
		// Thunks after forwards so they can call user functions.
		b.WriteString(e.deferThunks.String())
		b.WriteByte('\n')
	}
	if (e.needFunc || e.needGo) && e.funcTramps.Len() > 0 {
		b.WriteString(e.funcTramps.String())
		b.WriteByte('\n')
	}
	b.WriteString(body.String())

	b.WriteString("int main(void) {\n")
	if e.needArena {
		b.WriteString("\tuli_heap_init();\n")
	}
	for _, p := range pkgs {
		if e.pkgVarInitNeeded(p) {
			b.WriteString("\t")
			b.WriteString(cPkgIdent(p.Name, "__init_vars"))
			b.WriteString("();\n")
		}
	}
	b.WriteString("\t")
	b.WriteString(cPkgIdent(e.entry, "தொடக்கம்"))
	b.WriteString("();\n")
	if e.needGo {
		b.WriteString("\tuli_go_wait_all();\n")
	}
	if e.needArena {
		b.WriteString("\tuli_arena_free();\n")
	}
	b.WriteString("\treturn 0;\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func (e *emitter) pkgVarInitNeeded(p *check.PkgInfo) bool {
	if p == nil || p.File == nil {
		return false
	}
	for _, d := range p.File.Decls {
		switch d := d.(type) {
		case *ast.VarDecl:
			if len(d.Values) > 0 {
				return true
			}
		case *ast.VarGroupDecl:
			for _, spec := range d.Specs {
				if len(spec.Values) > 0 {
					return true
				}
			}
		}
	}
	return false
}

func (e *emitter) writePkgVarGlobals(b *strings.Builder, pkgs []*check.PkgInfo) {
	for _, p := range pkgs {
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		for _, d := range p.File.Decls {
			switch d := d.(type) {
			case *ast.VarDecl:
				e.writeOnePkgVarGlobal(b, p.Name, &ast.VarSpec{Names: d.Names, Type: d.Type, Values: d.Values})
			case *ast.VarGroupDecl:
				for _, spec := range d.Specs {
					e.writeOnePkgVarGlobal(b, p.Name, spec)
				}
			}
		}
	}
	if e.needPkgVarInit {
		b.WriteByte('\n')
	}
}

func (e *emitter) writeOnePkgVarGlobal(b *strings.Builder, pkg string, spec *ast.VarSpec) {
	if spec == nil {
		return
	}
	if spec.Type != nil && usesSliceType(spec.Type) {
		e.needSlice = true
	}
	if len(spec.Values) > 0 {
		e.needPkgVarInit = true
	}
	for i, name := range spec.Names {
		ct, zt := e.varSpecCType(pkg, name.Name, spec, i)
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(cPkgIdent(pkg, name.Name))
		b.WriteString(" = ")
		b.WriteString(zt)
		b.WriteString(";\n")
	}
}

func (e *emitter) varSpecCType(pkg, name string, spec *ast.VarSpec, i int) (cType, zero string) {
	if spec.Type != nil {
		return e.cTypeExpr(spec.Type), e.zeroInit(spec.Type)
	}
	if e.info != nil && e.info.PkgVarTypes != nil {
		if t, ok := e.info.PkgVarTypes[pkg+"."+name]; ok {
			return e.cTypeFrom(t), e.zeroInitType(t)
		}
	}
	if i < len(spec.Values) {
		t := e.typeOf(spec.Values[i])
		return e.cTypeFrom(t), e.zeroInitType(t)
	}
	return "void", "0"
}

func (e *emitter) writePkgVarInitFuncs(b *strings.Builder, pkgs []*check.PkgInfo) {
	for _, p := range pkgs {
		if !e.pkgVarInitNeeded(p) {
			continue
		}
		e.pkg = p.Name
		e.importLocal = p.ImportLocal
		b.WriteString("static void ")
		b.WriteString(cPkgIdent(p.Name, "__init_vars"))
		b.WriteString("(void) {\n")
		for _, d := range p.File.Decls {
			switch d := d.(type) {
			case *ast.VarDecl:
				e.writePkgVarInits(b, p.Name, d.Names, d.Values)
			case *ast.VarGroupDecl:
				for _, spec := range d.Specs {
					e.writePkgVarInits(b, p.Name, spec.Names, spec.Values)
				}
			}
		}
		b.WriteString("}\n\n")
	}
}

func (e *emitter) writePkgVarInits(b *strings.Builder, pkg string, names []*ast.Ident, values []ast.Expr) {
	for i, name := range names {
		if i >= len(values) {
			continue
		}
		b.WriteString("\t")
		b.WriteString(cPkgIdent(pkg, name.Name))
		b.WriteString(" = ")
		e.writeExpr(b, values[i])
		b.WriteString(";\n")
	}
}

func cPkgIdent(pkg, name string) string {
	if pkg == "" {
		return cIdent(name)
	}
	return cIdent(pkg + "__" + name)
}

func sortedStructTypes(info *check.Info) []check.Type {
	var types []check.Type
	for t := range info.Structs {
		types = append(types, t)
	}
	for i := 0; i < len(types); i++ {
		for j := i + 1; j < len(types); j++ {
			if types[j] < types[i] {
				types[i], types[j] = types[j], types[i]
			}
		}
	}
	return types
}

func sortedStructMethods(si *check.StructInfo) []*check.MethodInfo {
	if si == nil || si.Methods == nil {
		return nil
	}
	names := make([]string, 0, len(si.Methods))
	for name := range si.Methods {
		names = append(names, name)
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	out := make([]*check.MethodInfo, 0, len(names))
	for _, name := range names {
		out = append(out, si.Methods[name])
	}
	return out
}

func isSliceType(t check.Type) bool {
	return check.IsSlice(t)
}

func (e *emitter) sliceCName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_slice_i64"
	case check.TypeSliceBool:
		return "uli_slice_bool"
	case check.TypeSliceStr:
		return "uli_slice_str"
	case check.TypeSliceFloat:
		return "uli_slice_f64"
	case check.TypeSliceByte:
		return "uli_slice_u8"
	case check.TypeSliceRune:
		return "uli_slice_i32"
	default:
		return fmt.Sprintf("uli_slice_t%d", int(t))
	}
}

func (e *emitter) sliceGetName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_slice_get_i64"
	case check.TypeSliceBool:
		return "uli_slice_get_bool"
	case check.TypeSliceStr:
		return "uli_slice_get_str"
	case check.TypeSliceFloat:
		return "uli_slice_get_f64"
	case check.TypeSliceByte:
		return "uli_slice_get_u8"
	case check.TypeSliceRune:
		return "uli_slice_get_i32"
	default:
		return fmt.Sprintf("uli_slice_get_t%d", int(t))
	}
}

func (e *emitter) sliceSetName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_slice_set_i64"
	case check.TypeSliceBool:
		return "uli_slice_set_bool"
	case check.TypeSliceStr:
		return "uli_slice_set_str"
	case check.TypeSliceFloat:
		return "uli_slice_set_f64"
	case check.TypeSliceByte:
		return "uli_slice_set_u8"
	case check.TypeSliceRune:
		return "uli_slice_set_i32"
	default:
		return fmt.Sprintf("uli_slice_set_t%d", int(t))
	}
}

func (e *emitter) sliceSubName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_sub_i64"
	case check.TypeSliceBool:
		return "uli_sub_bool"
	case check.TypeSliceStr:
		return "uli_sub_str"
	case check.TypeSliceFloat:
		return "uli_sub_f64"
	case check.TypeSliceByte:
		return "uli_sub_u8"
	case check.TypeSliceRune:
		return "uli_sub_i32"
	default:
		return fmt.Sprintf("uli_sub_t%d", int(t))
	}
}

func (e *emitter) sliceSub3Name(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_sub3_i64"
	case check.TypeSliceBool:
		return "uli_sub3_bool"
	case check.TypeSliceStr:
		return "uli_sub3_str"
	case check.TypeSliceFloat:
		return "uli_sub3_f64"
	case check.TypeSliceByte:
		return "uli_sub3_u8"
	case check.TypeSliceRune:
		return "uli_sub3_i32"
	default:
		return fmt.Sprintf("uli_sub3_t%d", int(t))
	}
}

func (e *emitter) sliceAppendName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_append_i64"
	case check.TypeSliceBool:
		return "uli_append_bool"
	case check.TypeSliceStr:
		return "uli_append_str"
	case check.TypeSliceFloat:
		return "uli_append_f64"
	case check.TypeSliceByte:
		return "uli_append_u8"
	case check.TypeSliceRune:
		return "uli_append_i32"
	default:
		return fmt.Sprintf("uli_append_t%d", int(t))
	}
}

func (e *emitter) sliceElem(t check.Type) check.Type {
	return check.ElemOfSlice(e.info, t)
}

func (e *emitter) isLeafSlice(t check.Type) bool {
	return t == check.TypeSliceInt || t == check.TypeSliceBool || t == check.TypeSliceStr ||
		t == check.TypeSliceFloat || t == check.TypeSliceByte || t == check.TypeSliceRune
}

func (e *emitter) writeStructForwards(b *strings.Builder) {
	if e.info == nil || len(e.info.Structs) == 0 {
		return
	}
	for _, t := range sortedStructTypes(e.info) {
		si := e.info.Structs[t]
		if si.Schematic {
			continue
		}
		name := cPkgIdent(si.Pkg, si.Name)
		b.WriteString("typedef struct ")
		b.WriteString(name)
		b.WriteByte(' ')
		b.WriteString(name)
		b.WriteString(";\n")
	}
	b.WriteByte('\n')
}

func (e *emitter) writeStructBody(b *strings.Builder, t check.Type) {
	si := e.info.Structs[t]
	name := cPkgIdent(si.Pkg, si.Name)
	b.WriteString("struct ")
	b.WriteString(name)
	b.WriteString(" {\n")
	for _, f := range si.Fields {
		b.WriteString("\t")
		b.WriteString(e.cTypeFrom(f.Type))
		b.WriteByte(' ')
		b.WriteString(cIdent(f.Name))
		b.WriteString(";\n")
	}
	b.WriteString("};\n")
}

// writeStructsAndNestedSlices emits full struct bodies and nested slice helpers
// in dependency order so []Struct get/set/append see complete element types.
func (e *emitter) writeStructsAndNestedSlices(b *strings.Builder) {
	pendingStructs := map[check.Type]bool{}
	if e.info != nil {
		for t, si := range e.info.Structs {
			if si.Schematic {
				continue
			}
			pendingStructs[t] = true
		}
	}
	pendingSlices := map[check.Type]check.Type{}
	if e.info != nil {
		for t, el := range e.info.SliceElem {
			pendingSlices[t] = el
		}
	}
	doneStructs := map[check.Type]bool{}
	doneSlices := map[check.Type]bool{}

	for len(pendingStructs) > 0 || len(pendingSlices) > 0 {
		progress := false
		for t := range pendingStructs {
			if !e.structFieldsReady(t, doneStructs, doneSlices) {
				continue
			}
			e.writeStructBody(b, t)
			doneStructs[t] = true
			delete(pendingStructs, t)
			progress = true
		}
		for t, el := range pendingSlices {
			if !e.elemCompleteForSlice(el, doneStructs, doneSlices) {
				continue
			}
			e.writeOneNestedSliceRuntime(b, t)
			doneSlices[t] = true
			delete(pendingSlices, t)
			progress = true
		}
		if !progress {
			// Break cycles / unsupported deps: emit leftovers (may fail at cc).
			for t := range pendingStructs {
				e.writeStructBody(b, t)
				doneStructs[t] = true
				delete(pendingStructs, t)
			}
			for t := range pendingSlices {
				e.writeOneNestedSliceRuntime(b, t)
				doneSlices[t] = true
				delete(pendingSlices, t)
			}
		}
	}
	if len(doneStructs) > 0 {
		b.WriteByte('\n')
	}
	e.structsDone = true
}

func (e *emitter) structFieldsReady(t check.Type, doneStructs, doneSlices map[check.Type]bool) bool {
	si := e.info.Structs[t]
	for _, f := range si.Fields {
		if !e.typeReadyInStruct(f.Type, doneStructs, doneSlices) {
			return false
		}
	}
	return true
}

func (e *emitter) typeReadyInStruct(t check.Type, doneStructs, doneSlices map[check.Type]bool) bool {
	if e.info != nil {
		if _, ok := e.info.PtrElem[t]; ok {
			return true // pointer: forward decl is enough
		}
		if _, ok := e.info.Structs[t]; ok {
			return doneStructs[t]
		}
	}
	if e.isLeafSlice(t) {
		return true
	}
	if check.IsSlice(t) {
		return doneSlices[t]
	}
	// scalars, arrays (typedefs emitted later — only leaf-backed arrays used in structs today)
	return true
}

func (e *emitter) elemCompleteForSlice(el check.Type, doneStructs, doneSlices map[check.Type]bool) bool {
	if e.isLeafSlice(el) {
		return true
	}
	if check.IsSlice(el) {
		return doneSlices[el]
	}
	if e.info != nil {
		if _, ok := e.info.PtrElem[el]; ok {
			return true // []*T: incomplete pointee OK
		}
		if _, ok := e.info.Structs[el]; ok {
			return doneStructs[el] // []Struct: need complete type
		}
	}
	return true
}

func (e *emitter) elemReadyForSlice(el check.Type) bool {
	if e.isLeafSlice(el) {
		return true
	}
	if check.IsSlice(el) {
		return false // wait until that slice typedef is emitted
	}
	// After writeStructsAndNestedSlices, structs are complete; pointers/scalars OK.
	return true
}

func (e *emitter) nestedSliceOrder() []check.Type {
	if e.info == nil || len(e.info.SliceElem) == 0 {
		return nil
	}
	remaining := make(map[check.Type]check.Type, len(e.info.SliceElem))
	for t, el := range e.info.SliceElem {
		remaining[t] = el
	}
	var order []check.Type
	for len(remaining) > 0 {
		progress := false
		for t, el := range remaining {
			if e.elemReadyForSlice(el) || containsType(order, el) {
				order = append(order, t)
				delete(remaining, t)
				progress = true
			}
		}
		if !progress {
			for t := range remaining {
				order = append(order, t)
				delete(remaining, t)
			}
		}
	}
	return order
}

func containsType(ts []check.Type, want check.Type) bool {
	for _, t := range ts {
		if t == want {
			return true
		}
	}
	return false
}

func (e *emitter) writeOneNestedSliceRuntime(b *strings.Builder, t check.Type) {
	el := e.sliceElem(t)
	sn := e.sliceCName(t)
	en := e.cTypeFrom(el)
	fmt.Fprintf(b, "typedef struct { %s *data; int64_t len; int64_t cap; } %s;\n", en, sn)
	fmt.Fprintf(b, "static %s %s(%s s, int64_t i) {\n", en, e.sliceGetName(t), sn)
	b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\treturn s.data[i];\n}\n")
	fmt.Fprintf(b, "static void %s(%s s, int64_t i, %s v) {\n", e.sliceSetName(t), sn, en)
	b.WriteString("\tif (i < 0 || i >= s.len) abort();\n\ts.data[i] = v;\n}\n")
	fmt.Fprintf(b, "static %s %s(%s s, int64_t lo, int64_t hi) {\n", sn, e.sliceSubName(t), sn)
	b.WriteString("\tif (lo < 0 || hi < lo || hi > s.len) abort();\n")
	fmt.Fprintf(b, "\treturn (%s){ s.data + lo, hi - lo, s.cap - lo };\n}\n", sn)
	fmt.Fprintf(b, "static %s %s(%s s, int64_t lo, int64_t hi, int64_t max) {\n", sn, e.sliceSub3Name(t), sn)
	b.WriteString("\tif (lo < 0 || hi < lo || max < hi || max > s.cap) abort();\n")
	fmt.Fprintf(b, "\treturn (%s){ s.data + lo, hi - lo, max - lo };\n}\n", sn)
}

func (e *emitter) writeNestedSliceRuntime(b *strings.Builder) {
	for _, t := range e.nestedSliceOrder() {
		e.writeOneNestedSliceRuntime(b, t)
	}
}

func (e *emitter) writeNestedSliceAppend(b *strings.Builder) {
	for _, t := range e.nestedSliceOrder() {
		el := e.sliceElem(t)
		sn := e.sliceCName(t)
		en := e.cTypeFrom(el)
		fn := e.sliceAppendName(t)
		fmt.Fprintf(b, "static %s %s(%s s, %s v) {\n", sn, fn, sn, en)
		b.WriteString("\tint64_t ncap = s.cap;\n")
		b.WriteString("\tif (s.len + 1 > ncap) {\n")
		b.WriteString("\t\tncap = ncap < 4 ? 4 : ncap * 2;\n")
		b.WriteString("\t\twhile (ncap < s.len + 1) ncap *= 2;\n")
		fmt.Fprintf(b, "\t\t%s *nd = (%s *)uli_arena_alloc((size_t)ncap * sizeof(%s));\n", en, en, en)
		fmt.Fprintf(b, "\t\tif (s.len && s.data) memcpy(nd, s.data, (size_t)s.len * sizeof(%s));\n", en)
		b.WriteString("\t\ts.data = nd; s.cap = ncap;\n")
		b.WriteString("\t}\n\ts.data[s.len++] = v;\n\treturn s;\n}\n")
	}
}

func (e *emitter) makeName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_make_i64"
	case check.TypeSliceBool:
		return "uli_make_bool"
	case check.TypeSliceStr:
		return "uli_make_str"
	case check.TypeSliceFloat:
		return "uli_make_f64"
	case check.TypeSliceByte:
		return "uli_make_u8"
	case check.TypeSliceRune:
		return "uli_make_i32"
	default:
		return fmt.Sprintf("uli_make_t%d", int(t))
	}
}

func (e *emitter) copyName(t check.Type) string {
	switch t {
	case check.TypeSliceInt:
		return "uli_copy_i64"
	case check.TypeSliceBool:
		return "uli_copy_bool"
	case check.TypeSliceStr:
		return "uli_copy_str"
	case check.TypeSliceFloat:
		return "uli_copy_f64"
	case check.TypeSliceByte:
		return "uli_copy_u8"
	case check.TypeSliceRune:
		return "uli_copy_i32"
	default:
		return fmt.Sprintf("uli_copy_t%d", int(t))
	}
}

func (e *emitter) writeOneMake(b *strings.Builder, t check.Type) {
	sn := e.sliceCName(t)
	en := e.cTypeFrom(e.sliceElem(t))
	fn := e.makeName(t)
	fmt.Fprintf(b, "static %s %s(int64_t len, int64_t cap) {\n", sn, fn)
	b.WriteString("\tif (len < 0 || cap < len) abort();\n")
	fmt.Fprintf(b, "\tif (cap == 0) return (%s){0};\n", sn)
	fmt.Fprintf(b, "\t%s *d = (%s *)uli_arena_alloc((size_t)cap * sizeof(%s));\n", en, en, en)
	fmt.Fprintf(b, "\tmemset(d, 0, (size_t)cap * sizeof(%s));\n", en)
	fmt.Fprintf(b, "\treturn (%s){ d, len, cap };\n}\n", sn)
}

func (e *emitter) writeOneCopy(b *strings.Builder, t check.Type) {
	sn := e.sliceCName(t)
	en := e.cTypeFrom(e.sliceElem(t))
	fn := e.copyName(t)
	fmt.Fprintf(b, "static int64_t %s(%s dst, %s src) {\n", fn, sn, sn)
	b.WriteString("\tint64_t n = dst.len < src.len ? dst.len : src.len;\n")
	fmt.Fprintf(b, "\tif (n > 0) memcpy(dst.data, src.data, (size_t)n * sizeof(%s));\n", en)
	b.WriteString("\treturn n;\n}\n")
}

func (e *emitter) writeMakeRuntime(b *strings.Builder) {
	e.writeOneMake(b, check.TypeSliceInt)
	e.writeOneMake(b, check.TypeSliceBool)
	e.writeOneMake(b, check.TypeSliceStr)
	e.writeOneMake(b, check.TypeSliceFloat)
	e.writeOneMake(b, check.TypeSliceByte)
	e.writeOneMake(b, check.TypeSliceRune)
	for _, t := range e.nestedSliceOrder() {
		e.writeOneMake(b, t)
	}
}

func (e *emitter) writeCopyRuntime(b *strings.Builder) {
	e.writeOneCopy(b, check.TypeSliceInt)
	e.writeOneCopy(b, check.TypeSliceBool)
	e.writeOneCopy(b, check.TypeSliceStr)
	e.writeOneCopy(b, check.TypeSliceFloat)
	e.writeOneCopy(b, check.TypeSliceByte)
	e.writeOneCopy(b, check.TypeSliceRune)
	for _, t := range e.nestedSliceOrder() {
		e.writeOneCopy(b, t)
	}
}

func (e *emitter) markSliceNeedsFromStructs() {
	if e.info == nil {
		return
	}
	for _, si := range e.info.Structs {
		for _, f := range si.Fields {
			if isSliceType(f.Type) {
				e.needSlice = true
				return
			}
		}
	}
}

func (e *emitter) structComparable(t check.Type) bool {
	si, ok := e.info.Structs[t]
	if !ok {
		return false
	}
	for _, f := range si.Fields {
		if isSliceType(f.Type) || check.IsFunc(f.Type) || check.IsMap(f.Type) || check.IsChan(f.Type) {
			return false
		}
		if _, nested := e.info.Structs[f.Type]; nested {
			if !e.structComparable(f.Type) {
				return false
			}
		}
	}
	return true
}

func (e *emitter) eqFuncName(si *check.StructInfo) string {
	return "uli_eq_" + cPkgIdent(si.Pkg, si.Name)
}

func (e *emitter) writeStructEqFn(b *strings.Builder, t check.Type) {
	si := e.info.Structs[t]
	ty := cPkgIdent(si.Pkg, si.Name)
	fn := e.eqFuncName(si)
	b.WriteString("static int ")
	b.WriteString(fn)
	b.WriteByte('(')
	b.WriteString(ty)
	b.WriteString(" a, ")
	b.WriteString(ty)
	b.WriteString(" b) {\n")
	if len(si.Fields) == 0 {
		b.WriteString("\treturn 1;\n}\n\n")
		return
	}
	b.WriteString("\treturn ")
	for i, f := range si.Fields {
		if i > 0 {
			b.WriteString(" && ")
		}
		e.writeFieldEq(b, f)
	}
	b.WriteString(";\n}\n\n")
}

func (e *emitter) writeFieldEq(b *strings.Builder, f check.StructField) {
	e.writeValueEq(b, f.Type, "a."+cIdent(f.Name), "b."+cIdent(f.Name))
}

func (e *emitter) writeValueEq(b *strings.Builder, t check.Type, a, c string) {
	t = e.peelUnderlying(t)
	switch {
	case t == check.TypeString:
		fmt.Fprintf(b, "(strcmp(%s ? %s : \"\", %s ? %s : \"\") == 0)", a, a, c, c)
	case check.IsArray(t):
		fmt.Fprintf(b, "%s(%s, %s)", e.mapKeyEqName(t), a, c)
	case e.info != nil && e.info.Structs[t] != nil:
		fmt.Fprintf(b, "%s(%s, %s)", e.eqFuncName(e.info.Structs[t]), a, c)
	default:
		fmt.Fprintf(b, "(%s == %s)", a, c)
	}
}

func (e *emitter) writeArrayEqFuncs(b *strings.Builder) {
	if e.info == nil || len(e.info.Arrays) == 0 {
		return
	}
	var keys []check.Type
	for t := range e.info.Arrays {
		keys = append(keys, t)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, t := range keys {
		ai := e.info.Arrays[t]
		name := e.arrayCName(t)
		fmt.Fprintf(b, "static int %s_eq(%s a, %s b) {\n", name, name, name)
		fmt.Fprintf(b, "\tfor (int64_t i = 0; i < %dLL; i++) {\n", ai.Len)
		b.WriteString("\t\tif (!(")
		e.writeValueEq(b, ai.Elem, "a.data[i]", "b.data[i]")
		b.WriteString(")) return 0;\n")
		b.WriteString("\t}\n\treturn 1;\n}\n")
	}
}

func (e *emitter) writeMapKeyHelpers(b *strings.Builder) {
	seen := map[check.Type]bool{}
	for _, mt := range e.sortedMapTypes() {
		e.ensureMapKeyHelper(b, e.info.Maps[mt].Key, seen)
	}
}

func (e *emitter) ensureMapKeyHelper(b *strings.Builder, keyT check.Type, seen map[check.Type]bool) {
	keyT = e.peelUnderlying(keyT)
	if keyT == check.TypeInt || keyT == check.TypeBool || keyT == check.TypeString ||
		keyT == check.TypeFloat || keyT == check.TypeInvalid {
		return
	}
	if e.info != nil {
		if _, ok := e.info.PtrElem[keyT]; ok {
			return
		}
	}
	if seen[keyT] {
		return
	}
	if check.IsArray(keyT) {
		ai := e.info.Arrays[keyT]
		e.ensureMapKeyHelper(b, ai.Elem, seen)
		if seen[keyT] {
			return
		}
		seen[keyT] = true
		name := e.arrayCName(keyT)
		// _eq emitted by writeArrayEqFuncs; hash for map keys.
		fmt.Fprintf(b, "static uint64_t %s_hash(%s a) {\n", name, name)
		b.WriteString("\tuint64_t h = 14695981039346656037ULL;\n")
		fmt.Fprintf(b, "\tfor (int64_t i = 0; i < %dLL; i++) {\n", ai.Len)
		b.WriteString("\t\th ^= ")
		e.writeMapKeyHash(b, ai.Elem, "a.data[i]")
		b.WriteString(";\n\t\th *= 1099511628211ULL;\n")
		b.WriteString("\t}\n\treturn h;\n}\n")
		return
	}
	if e.info != nil {
		if si, ok := e.info.Structs[keyT]; ok {
			for _, f := range si.Fields {
				e.ensureMapKeyHelper(b, f.Type, seen)
			}
			seen[keyT] = true
			ty := cPkgIdent(si.Pkg, si.Name)
			fn := e.mapKeyHashName(keyT)
			fmt.Fprintf(b, "static uint64_t %s(%s a) {\n", fn, ty)
			b.WriteString("\tuint64_t h = 14695981039346656037ULL;\n")
			for _, f := range si.Fields {
				b.WriteString("\th ^= ")
				e.writeMapKeyHash(b, f.Type, "a."+cIdent(f.Name))
				b.WriteString(";\n\th *= 1099511628211ULL;\n")
			}
			b.WriteString("\treturn h;\n}\n")
		}
	}
}

func (e *emitter) printFuncName(si *check.StructInfo) string {
	return "uli_print_" + cPkgIdent(si.Pkg, si.Name)
}

func (e *emitter) writeStructPrintFn(b *strings.Builder, t check.Type) {
	si := e.info.Structs[t]
	ty := cPkgIdent(si.Pkg, si.Name)
	fn := e.printFuncName(si)
	b.WriteString("static void ")
	b.WriteString(fn)
	b.WriteByte('(')
	b.WriteString(ty)
	b.WriteString(" a) {\n")
	b.WriteString("\tprintf(\"")
	b.WriteString(escapeCString(si.Name))
	b.WriteString("{\");\n")
	for i, f := range si.Fields {
		if i > 0 {
			b.WriteString("\tprintf(\", \");\n")
		}
		b.WriteString("\tprintf(\"")
		b.WriteString(escapeCString(f.Name))
		b.WriteString(": \");\n")
		e.writeFieldPrint(b, "a."+cIdent(f.Name), f.Type)
	}
	b.WriteString("\tprintf(\"}\");\n")
	b.WriteString("}\n\n")
}

func (e *emitter) writeFieldPrint(b *strings.Builder, access string, t check.Type) {
	if e.info != nil {
		if under, ok := e.info.Underlying[t]; ok {
			e.writeFieldPrint(b, access, under)
			return
		}
	}
	switch {
	case t == check.TypeInt, t == check.TypeByte, t == check.TypeRune:
		b.WriteString("\tuli_print_int_n((int64_t)(")
		b.WriteString(access)
		b.WriteString("));\n")
	case t == check.TypeFloat:
		b.WriteString("\tuli_print_float_n(")
		b.WriteString(access)
		b.WriteString(");\n")
	case t == check.TypeBool:
		b.WriteString("\tuli_print_bool_n(")
		b.WriteString(access)
		b.WriteString(");\n")
	case t == check.TypeString:
		b.WriteString("\tuli_print_quoted_n(")
		b.WriteString(access)
		b.WriteString(");\n")
	case check.IsFunc(t):
		b.WriteString("\tprintf(\"<செயல்பாடு>\");\n")
	case e.info.Structs[t] != nil:
		b.WriteByte('\t')
		b.WriteString(e.printFuncName(e.info.Structs[t]))
		b.WriteByte('(')
		b.WriteString(access)
		b.WriteString(");\n")
	case isSliceType(t):
		e.needSlice = true
		e.writeSliceFieldPrint(b, access, t)
	default:
		// pointer
		b.WriteString("\tuli_print_ptr_n((const void *)(")
		b.WriteString(access)
		b.WriteString("));\n")
	}
}

// topoStructTypes orders structs so value-embedded fields are defined first.
func topoStructTypes(info *check.Info) []check.Type {
	all := sortedStructTypes(info)
	indeg := map[check.Type]int{}
	edges := map[check.Type][]check.Type{} // edge A→B means B embeds A by value
	for _, t := range all {
		indeg[t] = 0
	}
	for _, t := range all {
		si := info.Structs[t]
		seen := map[check.Type]bool{}
		for _, f := range si.Fields {
			if _, ok := info.Structs[f.Type]; !ok {
				continue
			}
			if seen[f.Type] {
				continue
			}
			seen[f.Type] = true
			edges[f.Type] = append(edges[f.Type], t)
			indeg[t]++
		}
	}
	var q []check.Type
	for _, t := range all {
		if indeg[t] == 0 {
			q = append(q, t)
		}
	}
	var out []check.Type
	for len(q) > 0 {
		t := q[0]
		q = q[1:]
		out = append(out, t)
		for _, next := range edges[t] {
			indeg[next]--
			if indeg[next] == 0 {
				q = append(q, next)
			}
		}
	}
	if len(out) != len(all) {
		// Cycle (should be rejected in check); fall back to id order.
		return all
	}
	return out
}

func cIdent(name string) string {
	var b strings.Builder
	b.WriteString("n_")
	for _, r := range name {
		fmt.Fprintf(&b, "%x_", r)
	}
	return b.String()
}

func (e *emitter) ptrTypeOf(elem check.Type) check.Type {
	if e.info == nil {
		return check.TypeInvalid
	}
	for pt, el := range e.info.PtrElem {
		if el == elem {
			return pt
		}
	}
	return check.TypeInvalid
}

func (e *emitter) cTypeExpr(te ast.TypeExpr) string {
	if te == nil {
		return "void"
	}
	switch te := te.(type) {
	case *ast.TypeName:
		if len(te.TypeArgs) > 0 {
			if t, ok := e.lookupInstantiatedType(te); ok {
				return e.cTypeFrom(t)
			}
		}
		if e.typeSubst != nil {
			if t, ok := e.typeSubst[te.Name]; ok {
				return e.cTypeFrom(t)
			}
		}
		switch te.Name {
		case "முழுஎண்", "முழுஎண்64":
			return "int64_t"
		case "நிலை":
			return "int"
		case "சரம்":
			return "const char *"
		case "மிதவைஎண்":
			return "double"
		case "இருமி8", "நேர்முழு8":
			return "uint8_t"
		case "இருமி32", "முழுஎண்32":
			return "int32_t"
		case "முழுஎண்8":
			return "int8_t"
		case "முழுஎண்16":
			return "int16_t"
		case "நேர்முழு16":
			return "uint16_t"
		case "நேர்முழு32":
			return "uint32_t"
		case "நேர்முழு64":
			return "uint64_t"
		default:
			if t, ok := e.lookupNamed(te); ok {
				return e.cTypeFrom(t)
			}
			return "void"
		}
	case *ast.SliceType:
		if t := e.resolveTypeExpr(te); check.IsSlice(t) {
			return e.sliceCName(t)
		}
		return "uli_slice_i64"
	case *ast.ArrayType:
		if t := e.resolveTypeExpr(te); check.IsArray(t) {
			return e.arrayCName(t)
		}
		return "int64_t"
	case *ast.PointerType:
		return e.cTypeExpr(te.Elem) + " *"
	case *ast.MapType:
		if t := e.resolveTypeExpr(te); check.IsMap(t) {
			return e.mapCName(t)
		}
		return "void *"
	case *ast.FuncType:
		e.needFunc = true
		return "uli_fn"
	default:
		return "void"
	}
}

func (e *emitter) resolveTypeExpr(te ast.TypeExpr) check.Type {
	if te == nil || e.info == nil {
		return check.TypeInvalid
	}
	switch te := te.(type) {
	case *ast.TypeName:
		if len(te.TypeArgs) > 0 {
			if t, ok := e.lookupInstantiatedType(te); ok {
				return t
			}
		}
		if e.typeSubst != nil {
			if t, ok := e.typeSubst[te.Name]; ok {
				return t
			}
		}
		if t, ok := e.lookupNamed(te); ok {
			return t
		}
		switch te.Name {
		case "முழுஎண்":
			return check.TypeInt
		case "நிலை":
			return check.TypeBool
		case "சரம்":
			return check.TypeString
		case "மிதவைஎண்":
			return check.TypeFloat
		case "இருமி8":
			return check.TypeByte
		case "இருமி32":
			return check.TypeRune
		case "முழுஎண்8":
			return check.TypeInt8
		case "முழுஎண்16":
			return check.TypeInt16
		case "முழுஎண்32":
			return check.TypeInt32
		case "முழுஎண்64":
			return check.TypeInt64
		case "நேர்முழு8":
			return check.TypeUint8
		case "நேர்முழு16":
			return check.TypeUint16
		case "நேர்முழு32":
			return check.TypeUint32
		case "நேர்முழு64":
			return check.TypeUint64
		}
		return check.TypeInvalid
	case *ast.SliceType:
		elem := e.resolveTypeExpr(te.Elem)
		return e.findSliceOf(elem)
	case *ast.ArrayType:
		elem := e.resolveTypeExpr(te.Elem)
		if e.info != nil {
			for t, ai := range e.info.Arrays {
				if ai.Len == te.Len && ai.Elem == elem {
					return t
				}
			}
		}
		return check.TypeInvalid
	case *ast.PointerType:
		elem := e.resolveTypeExpr(te.Elem)
		if elem == check.TypeInvalid {
			return check.TypeInvalid
		}
		for pt, el := range e.info.PtrElem {
			if el == elem {
				return pt
			}
		}
		return check.TypeInvalid
	case *ast.MapType:
		key := e.resolveTypeExpr(te.Key)
		elem := e.resolveTypeExpr(te.Elem)
		if e.info != nil {
			for t, mi := range e.info.Maps {
				if mi.Key == key && mi.Elem == elem {
					return t
				}
			}
		}
		return check.TypeInvalid
	case *ast.FuncType:
		params := make([]check.Type, len(te.Params))
		for i, p := range te.Params {
			params[i] = e.resolveTypeExpr(p)
		}
		var results []check.Type
		for _, r := range te.Results {
			results = append(results, e.resolveTypeExpr(r.Type))
		}
		if e.info != nil {
			for t, fi := range e.info.Funcs {
				if len(fi.Params) == len(params) && len(fi.Results) == len(results) {
					ok := true
					for i := range params {
						if fi.Params[i] != params[i] {
							ok = false
							break
						}
					}
					if ok {
						for i := range results {
							if fi.Results[i] != results[i] {
								ok = false
								break
							}
						}
					}
					if ok {
						return t
					}
				}
			}
		}
		return check.TypeInvalid
	default:
		return check.TypeInvalid
	}
}

func (e *emitter) findSliceOf(elem check.Type) check.Type {
	switch elem {
	case check.TypeInt:
		return check.TypeSliceInt
	case check.TypeBool:
		return check.TypeSliceBool
	case check.TypeString:
		return check.TypeSliceStr
	case check.TypeFloat:
		return check.TypeSliceFloat
	case check.TypeByte:
		return check.TypeSliceByte
	case check.TypeRune:
		return check.TypeSliceRune
	}
	if e.info != nil {
		for st, el := range e.info.SliceElem {
			if el == elem {
				return st
			}
		}
	}
	return check.TypeInvalid
}

func (e *emitter) cTypeFrom(t check.Type) string {
	if e.info != nil {
		if elem, ok := e.info.PtrElem[t]; ok {
			return e.cTypeFrom(elem) + " *"
		}
		if under, ok := e.info.Underlying[t]; ok {
			return e.cTypeFrom(under)
		}
		if si, ok := e.info.Structs[t]; ok {
			return cPkgIdent(si.Pkg, si.Name)
		}
	}
	if check.IsSlice(t) {
		return e.sliceCName(t)
	}
	if check.IsArray(t) {
		return e.arrayCName(t)
	}
	if check.IsMap(t) {
		return e.mapCName(t)
	}
	if check.IsFunc(t) {
		e.needFunc = true
		return "uli_fn"
	}
	if check.IsChan(t) {
		e.needChan = true
		return "uli_chan *"
	}
	if check.IsTuple(t) && e.info != nil {
		if elems, ok := e.info.TupleElems[t]; ok {
			return e.retCName(elems)
		}
	}
	switch t {
	case check.TypeInt, check.TypeInt64:
		return "int64_t"
	case check.TypeFloat:
		return "double"
	case check.TypeBool:
		return "int"
	case check.TypeString:
		return "const char *"
	case check.TypeByte, check.TypeUint8:
		return "uint8_t"
	case check.TypeRune, check.TypeInt32:
		return "int32_t"
	case check.TypeInt8:
		return "int8_t"
	case check.TypeInt16:
		return "int16_t"
	case check.TypeUint16:
		return "uint16_t"
	case check.TypeUint32:
		return "uint32_t"
	case check.TypeUint64:
		return "uint64_t"
	default:
		return "int64_t"
	}
}

func (e *emitter) writeSliceFieldPrint(b *strings.Builder, access string, t check.Type) {
	elem := e.sliceElem(t)
	b.WriteString("\tprintf(\"[\");\n")
	b.WriteString("\tfor (int64_t __i = 0; __i < ")
	b.WriteString(access)
	b.WriteString(".len; __i++) {\n")
	b.WriteString("\t\tif (__i) printf(\", \");\n")
	switch {
	case elem == check.TypeBool:
		fmt.Fprintf(b, "\t\tuli_print_bool_n(%s(%s, __i));\n", e.sliceGetName(t), access)
	case elem == check.TypeString:
		fmt.Fprintf(b, "\t\tuli_print_quoted_n(%s(%s, __i));\n", e.sliceGetName(t), access)
	case elem == check.TypeInt, elem == check.TypeByte, elem == check.TypeRune,
		elem == check.TypeInt8, elem == check.TypeInt16, elem == check.TypeInt32, elem == check.TypeInt64,
		elem == check.TypeUint8, elem == check.TypeUint16, elem == check.TypeUint32, elem == check.TypeUint64:
		fmt.Fprintf(b, "\t\tuli_print_int_n((int64_t)%s(%s, __i));\n", e.sliceGetName(t), access)
	case elem == check.TypeFloat:
		fmt.Fprintf(b, "\t\tuli_print_float_n(%s(%s, __i));\n", e.sliceGetName(t), access)
	case check.IsFunc(elem):
		b.WriteString("\t\tprintf(\"<செயல்பாடு>\");\n")
	case check.IsSlice(elem):
		tmp := fmt.Sprintf("__sl_%d", int(t))
		fmt.Fprintf(b, "\t\t{ %s %s = %s(%s, __i);\n", e.sliceCName(elem), tmp, e.sliceGetName(t), access)
		e.writeSliceFieldPrint(b, tmp, elem)
		b.WriteString("\t\t}\n")
	case e.info != nil && e.info.Structs[elem] != nil:
		fmt.Fprintf(b, "\t\t%s(%s(%s, __i));\n", e.printFuncName(e.info.Structs[elem]), e.sliceGetName(t), access)
	default:
		if e.info != nil {
			if _, ok := e.info.PtrElem[elem]; ok {
				fmt.Fprintf(b, "\t\tuli_print_ptr_n(%s(%s, __i));\n", e.sliceGetName(t), access)
				break
			}
		}
		fmt.Fprintf(b, "\t\tuli_print_int_n(%s(%s, __i));\n", e.sliceGetName(t), access)
	}
	b.WriteString("\t}\n")
	b.WriteString("\tprintf(\"]\");\n")
}

func (e *emitter) zeroInit(te ast.TypeExpr) string {
	switch te := te.(type) {
	case *ast.TypeName:
		if te.Name == "சரம்" {
			return "\"\""
		}
		if _, ok := e.lookupNamed(te); ok {
			return "{0}"
		}
		return "0"
	case *ast.SliceType, *ast.ArrayType, *ast.FuncType, *ast.MapType:
		return "{0}"
	case *ast.PointerType, *ast.ChanType:
		return "NULL"
	default:
		return "0"
	}
}

func (e *emitter) zeroInitType(t check.Type) string {
	t = e.peelUnderlying(t)
	switch {
	case t == check.TypeString:
		return "\"\""
	case check.IsSlice(t) || check.IsArray(t) || check.IsMap(t) || check.IsFunc(t):
		return "{0}"
	case e.info != nil && e.info.Structs[t] != nil:
		return "{0}"
	case e.info != nil && e.info.PtrElem[t] != 0:
		return "NULL"
	case check.IsChan(t):
		return "NULL"
	default:
		return "0"
	}
}

func (e *emitter) realPkg(local string) string {
	if e.importLocal != nil {
		if real, ok := e.importLocal[local]; ok {
			return real
		}
	}
	return local
}

func (e *emitter) lookupInstantiatedType(te *ast.TypeName) (check.Type, bool) {
	if te == nil || len(te.TypeArgs) == 0 {
		return check.TypeInvalid, false
	}
	pkg := e.pkg
	if te.Pkg != nil {
		pkg = e.realPkg(te.Pkg.Name)
	}
	key := te.Name
	for _, ta := range te.TypeArgs {
		t := e.resolveTypeExpr(ta)
		key += "__" + e.typeStr(t)
	}
	if pkg != "" {
		if t, ok := e.info.TypeByName[pkg+"."+key]; ok {
			return t, true
		}
	}
	return check.TypeInvalid, false
}

func (e *emitter) typeStr(t check.Type) string {
	if e.info == nil {
		return "?"
	}
	if si, ok := e.info.Structs[t]; ok {
		return si.Name
	}
	if di, ok := e.info.Defined[t]; ok {
		return di.Name
	}
	switch t {
	case check.TypeInt:
		return "முழுஎண்"
	case check.TypeBool:
		return "நிலை"
	case check.TypeString:
		return "சரம்"
	case check.TypeFloat:
		return "மிதவைஎண்"
	case check.TypeByte:
		return "இருமி8"
	case check.TypeRune:
		return "இருமி32"
	case check.TypeInt8:
		return "முழுஎண்8"
	case check.TypeInt16:
		return "முழுஎண்16"
	case check.TypeInt32:
		return "முழுஎண்32"
	case check.TypeInt64:
		return "முழுஎண்64"
	case check.TypeUint8:
		return "நேர்முழு8"
	case check.TypeUint16:
		return "நேர்முழு16"
	case check.TypeUint32:
		return "நேர்முழு32"
	case check.TypeUint64:
		return "நேர்முழு64"
	default:
		if check.IsSlice(t) {
			return "[]" + e.typeStr(check.ElemOfSlice(e.info, t))
		}
		if check.IsArray(t) {
			ai := e.info.Arrays[t]
			return fmt.Sprintf("[%d]%s", ai.Len, e.typeStr(ai.Elem))
		}
		return fmt.Sprintf("t%d", int(t))
	}
}

func (e *emitter) lookupNamed(te *ast.TypeName) (check.Type, bool) {
	if te == nil {
		return check.TypeInvalid, false
	}
	if len(te.TypeArgs) > 0 {
		return e.lookupInstantiatedType(te)
	}
	if te.Pkg != nil {
		t, ok := e.info.TypeByName[e.realPkg(te.Pkg.Name)+"."+te.Name]
		return t, ok
	}
	if e.pkg != "" {
		if t, ok := e.info.TypeByName[e.pkg+"."+te.Name]; ok {
			return t, true
		}
	}
	t, ok := e.info.TypeByName[te.Name]
	return t, ok
}

func (e *emitter) methodCName(fn *ast.FuncDecl) string {
	recvType := fn.Recv.Type
	if pt, ok := recvType.(*ast.PointerType); ok {
		recvType = pt.Elem
	}
	pkg := e.pkg
	typeName := "?"
	if tn, ok := recvType.(*ast.TypeName); ok {
		typeName = tn.Name
		if tn.Pkg != nil {
			pkg = tn.Pkg.Name
		}
		if t, ok := e.lookupNamed(tn); ok {
			if si, ok := e.info.Structs[t]; ok {
				pkg = si.Pkg
				typeName = si.Name
			}
		}
	}
	return cPkgIdent(pkg, typeName+"_"+fn.Name.Name)
}

func (e *emitter) resultTypes(fn *ast.FuncDecl) []check.Type {
	if len(fn.Results) == 0 {
		return nil
	}
	out := make([]check.Type, len(fn.Results))
	for i, r := range fn.Results {
		out[i] = e.resolveTypeExpr(r.Type)
		if check.IsSlice(out[i]) {
			e.needSlice = true
		}
	}
	return out
}

func (e *emitter) retTag(t check.Type) string {
	switch t {
	case check.TypeInt:
		return "i64"
	case check.TypeFloat:
		return "f64"
	case check.TypeBool:
		return "bool"
	case check.TypeString:
		return "str"
	case check.TypeByte:
		return "u8"
	case check.TypeRune:
		return "i32"
	default:
		if check.IsSlice(t) {
			return fmt.Sprintf("s%d", int(t))
		}
		if check.IsMap(t) {
			return fmt.Sprintf("m%d", int(t))
		}
		if e.info != nil {
			if si, ok := e.info.Structs[t]; ok {
				return cIdent(si.Name)
			}
			if _, ok := e.info.PtrElem[t]; ok {
				return fmt.Sprintf("p%d", int(t))
			}
		}
		return fmt.Sprintf("t%d", int(t))
	}
}

func (e *emitter) retCName(types []check.Type) string {
	var b strings.Builder
	b.WriteString("uli_ret")
	for _, t := range types {
		b.WriteByte('_')
		b.WriteString(e.retTag(t))
	}
	return b.String()
}

func (e *emitter) arrayCName(t check.Type) string {
	ai := e.info.Arrays[t]
	return fmt.Sprintf("uli_arr_%s_%d", e.retTag(ai.Elem), ai.Len)
}

func (e *emitter) mapCName(t check.Type) string {
	return fmt.Sprintf("uli_map_t%d", int(t))
}

func (e *emitter) mapTabName(t check.Type) string {
	return fmt.Sprintf("uli_map_tab_t%d", int(t))
}

func (e *emitter) mapEntName(t check.Type) string {
	return fmt.Sprintf("uli_map_ent_t%d", int(t))
}

func (e *emitter) mapMakeName(t check.Type) string {
	return e.mapCName(t) + "_make"
}

func (e *emitter) mapGetName(t check.Type) string {
	return e.mapCName(t) + "_get"
}

func (e *emitter) mapGetOkName(t check.Type) string {
	return e.mapCName(t) + "_get_ok"
}

func (e *emitter) mapSetName(t check.Type) string {
	return e.mapCName(t) + "_set"
}

func (e *emitter) mapDelName(t check.Type) string {
	return e.mapCName(t) + "_delete"
}

func (e *emitter) mapLenName(t check.Type) string {
	return e.mapCName(t) + "_len"
}

func (e *emitter) sortedMapTypes() []check.Type {
	if e.info == nil || len(e.info.Maps) == 0 {
		return nil
	}
	var keys []check.Type
	for t := range e.info.Maps {
		keys = append(keys, t)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func (e *emitter) mapKeyHashName(t check.Type) string {
	t = e.peelUnderlying(t)
	if check.IsArray(t) {
		return e.arrayCName(t) + "_hash"
	}
	if e.info != nil {
		if si, ok := e.info.Structs[t]; ok {
			return "uli_hash_" + cPkgIdent(si.Pkg, si.Name)
		}
	}
	return fmt.Sprintf("uli_hash_t%d", int(t))
}

func (e *emitter) mapKeyEqName(t check.Type) string {
	t = e.peelUnderlying(t)
	if check.IsArray(t) {
		return e.arrayCName(t) + "_eq"
	}
	if e.info != nil {
		if si, ok := e.info.Structs[t]; ok {
			return e.eqFuncName(si)
		}
	}
	return fmt.Sprintf("uli_eq_t%d", int(t))
}

func (e *emitter) writeMapKeyHash(b *strings.Builder, keyT check.Type, arg string) {
	keyT = e.peelUnderlying(keyT)
	switch {
	case keyT == check.TypeString:
		fmt.Fprintf(b, "uli_map_hash_str(%s)", arg)
	case keyT == check.TypeBool:
		fmt.Fprintf(b, "(uint64_t)(%s ? 1 : 0)", arg)
	case keyT == check.TypeFloat:
		fmt.Fprintf(b, "uli_map_hash_f64(%s)", arg)
	case keyT == check.TypeInt:
		fmt.Fprintf(b, "(uint64_t)%s", arg)
	case check.IsArray(keyT):
		fmt.Fprintf(b, "%s(%s)", e.mapKeyHashName(keyT), arg)
	case e.info != nil && e.info.Structs[keyT] != nil:
		fmt.Fprintf(b, "%s(%s)", e.mapKeyHashName(keyT), arg)
	default:
		if e.info != nil {
			if _, ok := e.info.PtrElem[keyT]; ok {
				fmt.Fprintf(b, "(uint64_t)(uintptr_t)(const void *)(%s)", arg)
				return
			}
		}
		fmt.Fprintf(b, "(uint64_t)%s", arg)
	}
}

func (e *emitter) writeMapKeyEq(b *strings.Builder, keyT check.Type, a, c string) {
	keyT = e.peelUnderlying(keyT)
	switch {
	case keyT == check.TypeString:
		fmt.Fprintf(b, "(strcmp(%s ? %s : \"\", %s ? %s : \"\") == 0)", a, a, c, c)
	case check.IsArray(keyT):
		fmt.Fprintf(b, "%s(%s, %s)", e.mapKeyEqName(keyT), a, c)
	case e.info != nil && e.info.Structs[keyT] != nil:
		fmt.Fprintf(b, "%s(%s, %s)", e.eqFuncName(e.info.Structs[keyT]), a, c)
	default:
		fmt.Fprintf(b, "(%s == %s)", a, c)
	}
}

func (e *emitter) writeMapForwards(b *strings.Builder) {
	keys := e.sortedMapTypes()
	if len(keys) == 0 {
		return
	}
	e.needMap = true
	e.needArena = true
	for _, t := range keys {
		mn := e.mapCName(t)
		tn := e.mapTabName(t)
		b.WriteString("typedef struct ")
		b.WriteString(tn)
		b.WriteByte(' ')
		b.WriteString(tn)
		b.WriteString(";\n")
		b.WriteString("typedef ")
		b.WriteString(tn)
		b.WriteString(" *")
		b.WriteString(mn)
		b.WriteString(";\n")
	}
}

func (e *emitter) writeMapFuncs(b *strings.Builder) {
	keys := e.sortedMapTypes()
	if len(keys) == 0 {
		return
	}
	e.needMap = true
	e.needArena = true
	b.WriteString("static uint64_t uli_map_hash_str(const char *s) {\n")
	b.WriteString("\tuint64_t h = 14695981039346656037ULL;\n")
	b.WriteString("\tif (!s) return h;\n")
	b.WriteString("\tfor (const unsigned char *p = (const unsigned char *)s; *p; p++) {\n")
	b.WriteString("\t\th ^= (uint64_t)(*p);\n")
	b.WriteString("\t\th *= 1099511628211ULL;\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn h;\n")
	b.WriteString("}\n")
	b.WriteString("static uint64_t uli_map_hash_f64(double x) {\n")
	b.WriteString("\tuint64_t u; memcpy(&u, &x, sizeof(u));\n")
	b.WriteString("\tif (u == 0x8000000000000000ULL) u = 0; /* -0 == +0 */\n")
	b.WriteString("\treturn u;\n")
	b.WriteString("}\n")
	e.writeMapKeyHelpers(b)
	for _, t := range keys {
		mi := e.info.Maps[t]
		mn := e.mapCName(t)
		tn := e.mapTabName(t)
		en := e.mapEntName(t)
		kt := e.cTypeFrom(mi.Key)
		vt := e.cTypeFrom(mi.Elem)
		zero := e.zeroCExpr(mi.Elem)
		okRet := e.retCName([]check.Type{mi.Elem, check.TypeBool})

		fmt.Fprintf(b, "typedef struct { int state; %s key; %s val; } %s;\n", kt, vt, en)
		fmt.Fprintf(b, "struct %s { int64_t len; int64_t cap; %s *entries; };\n", tn, en)

		fmt.Fprintf(b, "static %s %s(int64_t hint) {\n", mn, e.mapMakeName(t))
		b.WriteString("\tint64_t cap = 8;\n")
		b.WriteString("\tif (hint > cap) {\n")
		b.WriteString("\t\tcap = 1;\n")
		b.WriteString("\t\twhile (cap < hint) cap *= 2;\n")
		b.WriteString("\t}\n")
		fmt.Fprintf(b, "\t%s *tab = (%s *)uli_arena_alloc(sizeof(%s));\n", tn, tn, tn)
		b.WriteString("\ttab->len = 0; tab->cap = cap;\n")
		fmt.Fprintf(b, "\ttab->entries = (%s *)uli_arena_alloc((size_t)cap * sizeof(%s));\n", en, en)
		b.WriteString("\tmemset(tab->entries, 0, (size_t)cap * sizeof(*tab->entries));\n")
		b.WriteString("\treturn tab;\n}\n")

		fmt.Fprintf(b, "static int64_t %s(%s m) { return m ? m->len : 0LL; }\n", e.mapLenName(t), mn)

		fmt.Fprintf(b, "static void %s_rehash(%s m, int64_t ncap) {\n", mn, mn)
		fmt.Fprintf(b, "\t%s *old = m->entries;\n", en)
		b.WriteString("\tint64_t ocap = m->cap;\n")
		fmt.Fprintf(b, "\t%s *neu = (%s *)uli_arena_alloc((size_t)ncap * sizeof(%s));\n", en, en, en)
		b.WriteString("\tmemset(neu, 0, (size_t)ncap * sizeof(*neu));\n")
		b.WriteString("\tm->entries = neu; m->cap = ncap; m->len = 0;\n")
		b.WriteString("\tfor (int64_t i = 0; i < ocap; i++) {\n")
		b.WriteString("\t\tif (old[i].state != 1) continue;\n")
		b.WriteString("\t\tuint64_t h = ")
		e.writeMapKeyHash(b, mi.Key, "old[i].key")
		b.WriteString(";\n")
		b.WriteString("\t\tuint64_t j = h & (uint64_t)(ncap - 1);\n")
		b.WriteString("\t\twhile (neu[j].state == 1) j = (j + 1) & (uint64_t)(ncap - 1);\n")
		b.WriteString("\t\tneu[j].state = 1; neu[j].key = old[i].key; neu[j].val = old[i].val; m->len++;\n")
		b.WriteString("\t}\n}\n")

		fmt.Fprintf(b, "static %s %s(%s m, %s k) {\n", vt, e.mapGetName(t), mn, kt)
		fmt.Fprintf(b, "\tif (!m) return %s;\n", zero)
		b.WriteString("\tuint64_t h = ")
		e.writeMapKeyHash(b, mi.Key, "k")
		b.WriteString(";\n")
		b.WriteString("\tuint64_t j = h & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\tfor (int64_t n = 0; n < m->cap; n++) {\n")
		b.WriteString("\t\tif (m->entries[j].state == 0) break;\n")
		b.WriteString("\t\tif (m->entries[j].state == 1 && ")
		e.writeMapKeyEq(b, mi.Key, "m->entries[j].key", "k")
		b.WriteString(") return m->entries[j].val;\n")
		b.WriteString("\t\tj = (j + 1) & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\t}\n")
		fmt.Fprintf(b, "\treturn %s;\n}\n", zero)

		fmt.Fprintf(b, "static %s %s(%s m, %s k) {\n", okRet, e.mapGetOkName(t), mn, kt)
		fmt.Fprintf(b, "\t%s r; r.r0 = %s; r.r1 = 0;\n", okRet, zero)
		b.WriteString("\tif (!m) return r;\n")
		b.WriteString("\tuint64_t h = ")
		e.writeMapKeyHash(b, mi.Key, "k")
		b.WriteString(";\n")
		b.WriteString("\tuint64_t j = h & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\tfor (int64_t n = 0; n < m->cap; n++) {\n")
		b.WriteString("\t\tif (m->entries[j].state == 0) break;\n")
		b.WriteString("\t\tif (m->entries[j].state == 1 && ")
		e.writeMapKeyEq(b, mi.Key, "m->entries[j].key", "k")
		b.WriteString(") { r.r0 = m->entries[j].val; r.r1 = 1; return r; }\n")
		b.WriteString("\t\tj = (j + 1) & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn r;\n}\n")

		fmt.Fprintf(b, "static void %s(%s m, %s k, %s v) {\n", e.mapSetName(t), mn, kt, vt)
		b.WriteString("\tif (!m) { fprintf(stderr, \"assignment to entry in nil map\\n\"); abort(); }\n")
		b.WriteString("\tif (m->len*2 >= m->cap) ")
		fmt.Fprintf(b, "%s_rehash(m, m->cap*2);\n", mn)
		b.WriteString("\tuint64_t h = ")
		e.writeMapKeyHash(b, mi.Key, "k")
		b.WriteString(";\n")
		b.WriteString("\tuint64_t j = h & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\tint64_t tomb = -1;\n")
		b.WriteString("\tfor (;;) {\n")
		b.WriteString("\t\tif (m->entries[j].state == 0) {\n")
		b.WriteString("\t\t\tint64_t slot = tomb >= 0 ? tomb : (int64_t)j;\n")
		b.WriteString("\t\t\tm->entries[slot].state = 1; m->entries[slot].key = k; m->entries[slot].val = v; m->len++; return;\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif (m->entries[j].state == 2) { if (tomb < 0) tomb = (int64_t)j; }\n")
		b.WriteString("\t\telse if (")
		e.writeMapKeyEq(b, mi.Key, "m->entries[j].key", "k")
		b.WriteString(") { m->entries[j].val = v; return; }\n")
		b.WriteString("\t\tj = (j + 1) & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\t}\n}\n")

		fmt.Fprintf(b, "static void %s(%s m, %s k) {\n", e.mapDelName(t), mn, kt)
		b.WriteString("\tif (!m) return;\n")
		b.WriteString("\tuint64_t h = ")
		e.writeMapKeyHash(b, mi.Key, "k")
		b.WriteString(";\n")
		b.WriteString("\tuint64_t j = h & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\tfor (int64_t n = 0; n < m->cap; n++) {\n")
		b.WriteString("\t\tif (m->entries[j].state == 0) return;\n")
		b.WriteString("\t\tif (m->entries[j].state == 1 && ")
		e.writeMapKeyEq(b, mi.Key, "m->entries[j].key", "k")
		b.WriteString(") { m->entries[j].state = 2; m->len--; return; }\n")
		b.WriteString("\t\tj = (j + 1) & (uint64_t)(m->cap - 1);\n")
		b.WriteString("\t}\n}\n")
	}
}

// zeroCExpr returns a C expression for the zero value of t (for map misses).
func (e *emitter) zeroCExpr(t check.Type) string {
	if e.info != nil {
		if under, ok := e.info.Underlying[t]; ok {
			return e.zeroCExpr(under)
		}
		if _, ok := e.info.PtrElem[t]; ok {
			return "NULL"
		}
		if e.info.Structs[t] != nil {
			return "(" + e.cTypeFrom(t) + "){0}"
		}
	}
	switch {
	case t == check.TypeString:
		return "\"\""
	case t == check.TypeFloat:
		return "0.0"
	case check.IsFunc(t):
		return "(uli_fn){0}"
	case check.IsSlice(t), check.IsArray(t):
		return "(" + e.cTypeFrom(t) + "){0}"
	case check.IsMap(t):
		return "NULL"
	default:
		return "0"
	}
}

func (e *emitter) writeArrayTypedefs(b *strings.Builder) {
	if e.info == nil || len(e.info.Arrays) == 0 {
		return
	}
	var keys []check.Type
	for t := range e.info.Arrays {
		keys = append(keys, t)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	// Emit shorter arrays / leaf elems first (nested arrays depend on elem typedefs).
	remaining := make(map[check.Type]check.ArrayInfo, len(keys))
	for _, t := range keys {
		remaining[t] = e.info.Arrays[t]
	}
	var order []check.Type
	for len(remaining) > 0 {
		progress := false
		for t, ai := range remaining {
			if !check.IsArray(ai.Elem) || containsType(order, ai.Elem) {
				order = append(order, t)
				delete(remaining, t)
				progress = true
			}
		}
		if !progress {
			for t := range remaining {
				order = append(order, t)
				delete(remaining, t)
			}
		}
	}
	seen := map[string]bool{}
	for _, t := range order {
		ai := e.info.Arrays[t]
		name := e.arrayCName(t)
		if seen[name] {
			continue
		}
		seen[name] = true
		en := e.cTypeFrom(ai.Elem)
		fmt.Fprintf(b, "typedef struct { %s data[%d]; } %s;\n", en, ai.Len, name)
		fmt.Fprintf(b, "static %s %s_get(%s a, int64_t i) {\n", en, name, name)
		fmt.Fprintf(b, "\tif (i < 0 || i >= %d) abort();\n\treturn a.data[i];\n}\n", ai.Len)
		fmt.Fprintf(b, "static void %s_set(%s *a, int64_t i, %s v) {\n", name, name, en)
		fmt.Fprintf(b, "\tif (i < 0 || i >= %d) abort();\n\ta->data[i] = v;\n}\n", ai.Len)
		if e.needSlice {
			if st := e.findSliceOf(ai.Elem); check.IsSlice(st) {
				sn := e.sliceCName(st)
				fmt.Fprintf(b, "static %s %s_slice(%s *a, int64_t lo, int64_t hi, int64_t max) {\n", sn, name, name)
				fmt.Fprintf(b, "\tif (lo < 0 || hi < lo || max < hi || max > %d) abort();\n", ai.Len)
				fmt.Fprintf(b, "\treturn (%s){ a->data + lo, hi - lo, max - lo };\n}\n", sn)
			}
		}
	}
}

func (e *emitter) writeTupleTypedefs(b *strings.Builder) {
	if e.info == nil {
		return
	}
	var keys []check.Type
	for t := range e.info.TupleElems {
		keys = append(keys, t)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	seen := map[string]bool{}
	for _, t := range keys {
		elems := e.info.TupleElems[t]
		name := e.retCName(elems)
		if seen[name] {
			continue
		}
		seen[name] = true
		b.WriteString("typedef struct { ")
		for i, el := range elems {
			if check.IsSlice(el) {
				e.needSlice = true
			}
			fmt.Fprintf(b, "%s r%d; ", e.cTypeFrom(el), i)
		}
		fmt.Fprintf(b, "} %s;\n", name)
	}
	// Imported intrinsic declarations may never appear as an expression tuple
	// in the entry package, but their emitted C signatures still need a tuple
	// typedef. Emit every declared multi-result function shape as well.
	for _, fi := range e.info.Funcs {
		if len(fi.Results) < 2 {
			continue
		}
		name := e.retCName(fi.Results)
		if seen[name] {
			continue
		}
		seen[name] = true
		b.WriteString("typedef struct { ")
		for i, el := range fi.Results {
			if check.IsSlice(el) {
				e.needSlice = true
			}
			fmt.Fprintf(b, "%s r%d; ", e.cTypeFrom(el), i)
		}
		fmt.Fprintf(b, "} %s;\n", name)
	}
	for _, elems := range e.tupleDecls {
		name := e.retCName(elems)
		if seen[name] {
			continue
		}
		seen[name] = true
		b.WriteString("typedef struct { ")
		for i, el := range elems {
			if check.IsSlice(el) {
				e.needSlice = true
			}
			fmt.Fprintf(b, "%s r%d; ", e.cTypeFrom(el), i)
		}
		fmt.Fprintf(b, "} %s;\n", name)
	}
}

func (e *emitter) monoCName(inst *check.MonoInst) string {
	return cPkgIdent(inst.Pkg, inst.Key)
}

func (e *emitter) cMonoFuncSig(inst *check.MonoInst) string {
	prevSubst := e.typeSubst
	prevName := e.monoName
	e.typeSubst = map[string]check.Type{}
	for i, name := range inst.Decl.TypeParams {
		if i < len(inst.TypeArgs) && name != nil && name.Name != nil {
			e.typeSubst[name.Name.Name] = inst.TypeArgs[i]
		}
	}
	e.monoName = e.monoCName(inst)
	sig := e.cFuncSig(inst.Decl)
	e.typeSubst = prevSubst
	e.monoName = prevName
	return sig
}

func (e *emitter) writeMonoFunc(b *strings.Builder, inst *check.MonoInst) {
	if inst == nil || inst.Decl == nil {
		return
	}
	prevSubst := e.typeSubst
	prevName := e.monoName
	e.typeSubst = map[string]check.Type{}
	for i, name := range inst.Decl.TypeParams {
		if i < len(inst.TypeArgs) && name != nil && name.Name != nil {
			e.typeSubst[name.Name.Name] = inst.TypeArgs[i]
		}
	}
	e.monoName = e.monoCName(inst)
	e.writeFunc(b, inst.Decl)
	e.typeSubst = prevSubst
	e.monoName = prevName
}

func (e *emitter) writeMonoMethod(b *strings.Builder, si *check.StructInfo, mi *check.MethodInfo) {
	if si == nil || mi == nil || mi.Decl == nil {
		return
	}
	prevSubst := e.typeSubst
	prevName := e.monoName
	e.typeSubst = map[string]check.Type{}
	for i, name := range si.GenericParamNames {
		if i < len(si.GenericTypeArgs) {
			e.typeSubst[name] = si.GenericTypeArgs[i]
		}
	}
	e.monoName = cPkgIdent(si.Pkg, si.Name+"_"+mi.Name)
	e.writeFunc(b, mi.Decl)
	e.typeSubst = prevSubst
	e.monoName = prevName
}

func (e *emitter) cFuncSig(fn *ast.FuncDecl) string {
	var b strings.Builder
	rts := e.resultTypes(fn)
	switch len(rts) {
	case 0:
		b.WriteString("void")
	case 1:
		b.WriteString(e.cTypeFrom(rts[0]))
	default:
		b.WriteString(e.retCName(rts))
	}
	b.WriteByte(' ')
	if e.monoName != "" {
		b.WriteString(e.monoName)
	} else if fn.Recv != nil {
		b.WriteString(e.methodCName(fn))
	} else {
		b.WriteString(cPkgIdent(e.pkg, fn.Name.Name))
	}
	b.WriteByte('(')
	nparams := len(fn.Params)
	if fn.Recv != nil {
		nparams++
	}
	if nparams == 0 {
		b.WriteString("void")
	} else {
		first := true
		if fn.Recv != nil {
			b.WriteString(e.cTypeExpr(fn.Recv.Type))
			b.WriteByte(' ')
			b.WriteString(cIdent(fn.Recv.Name.Name))
			first = false
		}
		for _, p := range fn.Params {
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(e.cParamType(p))
			b.WriteByte(' ')
			b.WriteString(cIdent(p.Name.Name))
		}
	}
	b.WriteByte(')')
	return b.String()
}

func (e *emitter) cParamType(p *ast.Field) string {
	if p != nil && p.Ellipsis {
		elem := e.resolveTypeExpr(p.Type)
		st := e.sliceTypeOfElem(elem)
		if st != check.TypeInvalid {
			return e.cTypeFrom(st)
		}
	}
	return e.cTypeExpr(p.Type)
}

func (e *emitter) sliceTypeOfElem(elem check.Type) check.Type {
	elem = e.peelUnderlying(elem)
	switch elem {
	case check.TypeInt:
		return check.TypeSliceInt
	case check.TypeBool:
		return check.TypeSliceBool
	case check.TypeString:
		return check.TypeSliceStr
	case check.TypeFloat:
		return check.TypeSliceFloat
	case check.TypeByte:
		return check.TypeSliceByte
	case check.TypeRune:
		return check.TypeSliceRune
	}
	if e.info != nil {
		for st, el := range e.info.SliceElem {
			if el == elem {
				return st
			}
		}
	}
	return check.TypeInvalid
}

func (e *emitter) writeCallArgs(b *strings.Builder, call *ast.CallExpr) {
	if call == nil {
		return
	}
	var pack *check.VariadicPack
	if e.info != nil {
		pack = e.info.VariadicPacks[call]
	}
	if pack == nil {
		for i, a := range call.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			e.writeExpr(b, a)
		}
		return
	}
	for i := 0; i < pack.Fixed; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		e.writeExpr(b, call.Args[i])
	}
	if pack.Fixed > 0 {
		b.WriteString(", ")
	}
	if pack.Expand {
		e.writeExpr(b, call.Args[len(call.Args)-1])
		return
	}
	e.writeVariadicSlice(b, call.Args[pack.Fixed:], pack)
}

func (e *emitter) writeVariadicSlice(b *strings.Builder, args []ast.Expr, pack *check.VariadicPack) {
	e.needSlice = true
	e.needArena = true
	sliceTy := e.sliceCName(pack.Slice)
	arrTy := e.cTypeFrom(pack.Elem)
	fmt.Fprintf(b, "(%s){ (%s[]){", sliceTy, arrTy)
	if len(args) == 0 {
		b.WriteString(e.zeroCValue(pack.Elem))
	} else {
		for i, a := range args {
			if i > 0 {
				b.WriteString(", ")
			}
			e.writeExpr(b, a)
		}
	}
	fmt.Fprintf(b, "}, %dLL, %dLL }", len(args), len(args))
}

func (e *emitter) writeFunc(b *strings.Builder, fn *ast.FuncDecl) {
	if e.writeNetIntrinsic(b, fn) {
		return
	}
	if e.writeHttpIntrinsic(b, fn) {
		return
	}
	if e.writeDBIntrinsic(b, fn) {
		return
	}
	if e.writeFileIntrinsic(b, fn) {
		return
	}
	if e.writeTimeIntrinsic(b, fn) {
		return
	}
	if e.writeFmtIntrinsic(b, fn) {
		return
	}
	prev := e.curFn
	prevDefer := e.fnHasDefer
	prevPromo := e.promoted
	e.curFn = fn
	e.fnHasDefer = hasDeferInFunc(fn)
	e.promoted = map[string]check.Type{}
	if e.info != nil {
		for name, t := range e.info.PromoteInFunc[fn] {
			e.promoted[name] = t
		}
	}
	defer func() {
		e.curFn = prev
		e.fnHasDefer = prevDefer
		e.promoted = prevPromo
	}()

	for _, r := range fn.Results {
		if usesSliceType(r.Type) {
			e.needSlice = true
		}
	}
	if fn.Recv != nil && usesSliceType(fn.Recv.Type) {
		e.needSlice = true
	}
	for _, p := range fn.Params {
		if usesSliceType(p.Type) {
			e.needSlice = true
		}
	}
	if e.fnHasDefer {
		e.needDefer = true
		e.needArena = true
	}
	if len(e.promoted) > 0 {
		e.ensureFuncRuntime()
	}
	b.WriteString(e.cFuncSig(fn))
	b.WriteString(" {\n")
	// Arena-promote captured params / receiver.
	if fn.Recv != nil && e.isPromoted(fn.Recv.Name.Name) {
		ct := e.cTypeExpr(fn.Recv.Type)
		fmt.Fprintf(b, "\t%s *%s = (%s *)uli_arena_alloc(sizeof(%s));\n", ct, e.capIdent(fn.Recv.Name.Name), ct, ct)
		fmt.Fprintf(b, "\t*%s = %s;\n", e.capIdent(fn.Recv.Name.Name), cIdent(fn.Recv.Name.Name))
	}
	for _, p := range fn.Params {
		if p.Name == nil || !e.isPromoted(p.Name.Name) {
			continue
		}
		ct := e.cTypeExpr(p.Type)
		fmt.Fprintf(b, "\t%s *%s = (%s *)uli_arena_alloc(sizeof(%s));\n", ct, e.capIdent(p.Name.Name), ct, ct)
		fmt.Fprintf(b, "\t*%s = %s;\n", e.capIdent(p.Name.Name), cIdent(p.Name.Name))
	}
	for _, r := range fn.Results {
		if r.Name == nil {
			continue
		}
		b.WriteByte('\t')
		b.WriteString(e.cTypeExpr(r.Type))
		b.WriteByte(' ')
		b.WriteString(cIdent(r.Name.Name))
		b.WriteString(" = ")
		b.WriteString(e.zeroInit(r.Type))
		b.WriteString(";\n")
	}
	e.writeUnwindPrologue(b, fn, e.panicFrameName(fn))
	e.writeGCPoll(b, 1)
	e.writeBlock(b, fn.Body, 1)
	if e.fnHasUnwind() {
		e.writeDeferEpilogue(b, fn)
	}
	b.WriteString("}\n")
}

func hasDeferInFunc(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	return hasDeferStmt(fn.Body)
}

func hasDeferStmt(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.DeferStmt:
		return true
	case *ast.BlockStmt:
		for _, x := range s.List {
			if hasDeferStmt(x) {
				return true
			}
		}
	case *ast.IfStmt:
		if hasDeferStmt(s.Body) {
			return true
		}
		if s.Else != nil && hasDeferStmt(s.Else) {
			return true
		}
		if s.Init != nil && hasDeferStmt(s.Init) {
			return true
		}
	case *ast.ForStmt:
		if s.Init != nil && hasDeferStmt(s.Init) {
			return true
		}
		if s.Post != nil && hasDeferStmt(s.Post) {
			return true
		}
		return hasDeferStmt(s.Body)
	case *ast.RangeStmt:
		return hasDeferStmt(s.Body)
	case *ast.SwitchStmt:
		if s.Init != nil && hasDeferStmt(s.Init) {
			return true
		}
		for _, c := range s.Cases {
			if c.Body != nil && hasDeferStmt(c.Body) {
				return true
			}
		}
	}
	return false
}

func (e *emitter) fnNamedResults() bool {
	if e.curFn == nil || len(e.curFn.Results) == 0 {
		return false
	}
	for _, r := range e.curFn.Results {
		if r == nil || r.Name == nil {
			return false
		}
	}
	return true
}

func (e *emitter) writeDeferEpilogue(b *strings.Builder, fn *ast.FuncDecl) {
	b.WriteString("__uli_epilogue:;\n")
	if e.fnHasDefer {
		b.WriteString("\twhile (_defers) {\n")
		b.WriteString("\t\tuli_defer_frame *_df = _defers;\n")
		b.WriteString("\t\t_defers = _df->next;\n")
		if e.needPanic {
			b.WriteString("\t\tuli_defer_pending = 1;\n")
			b.WriteString("\t\tuli_is_defer_fn = 1;\n")
		}
		b.WriteString("\t\t_df->fn(_df->arg);\n")
		if e.needPanic {
			b.WriteString("\t\tuli_defer_pending = 0;\n")
			b.WriteString("\t\tuli_is_defer_fn = 0;\n")
		}
		b.WriteString("\t}\n")
	}
	e.writePanicRethrow(b)
	rts := e.resultTypes(fn)
	b.WriteString("\treturn")
	if len(rts) == 0 {
		b.WriteString(";\n")
		return
	}
	b.WriteByte(' ')
	if e.fnNamedResults() {
		if len(rts) == 1 {
			b.WriteString(cIdent(fn.Results[0].Name.Name))
		} else {
			fmt.Fprintf(b, "(%s){", e.retCName(rts))
			for i, r := range fn.Results {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(cIdent(r.Name.Name))
			}
			b.WriteByte('}')
		}
	} else {
		b.WriteString("_ret")
	}
	b.WriteString(";\n")
}

type deferField struct {
	name     string
	ct       string
	expr     ast.Expr
	takeAddr bool // capture &expr (pointer method on addressable value)
}

func (e *emitter) writeDefer(b *strings.Builder, s *ast.DeferStmt, level int) {
	if s.Call == nil {
		return
	}
	e.needDefer = true
	e.needArena = true
	id := e.deferID
	e.deferID++
	thunk := fmt.Sprintf("uli_dthunk_%d", id)

	var fields []deferField
	add := func(expr ast.Expr) {
		t := e.typeOf(expr)
		ct := e.cTypeFrom(t)
		if strings.HasPrefix(ct, "uli_slice_") {
			e.needSlice = true
		}
		fields = append(fields, deferField{
			name: fmt.Sprintf("a%d", len(fields)),
			ct:   ct,
			expr: expr,
		})
	}

	call := s.Call
	for _, a := range call.Args {
		add(a)
	}
	var recvExpr ast.Expr
	var recvIsPtr bool
	var method *check.MethodInfo
	var methodStruct *check.StructInfo
	builtinName := ""
	pkgFunc := ""
	localFunc := ""
	fnValue := false

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if call.Builtin {
			builtinName = fun.Name
		} else {
			localFunc = fun.Name
		}
	case *ast.SelectorExpr:
		xt := e.typeOf(fun.X)
		if xt == check.TypeInvalid {
			if id, ok := fun.X.(*ast.Ident); ok {
				pkgFunc = cPkgIdent(e.realPkg(id.Name), fun.Sel.Name)
			}
		} else {
			base := xt
			if elem, ok := e.info.PtrElem[xt]; ok {
				base = elem
				recvIsPtr = true
			}
			if si := e.info.Structs[base]; si != nil {
				if mi := si.Methods[fun.Sel.Name]; mi != nil {
					method = mi
					methodStruct = si
					recvExpr = fun.X
					// Prepend receiver capture.
					// Pointer method on a value: save &x now (Go); later mutations are visible.
					// Value method: copy the receiver value now.
					captureT := xt
					takeAddr := false
					if mi.RecvIsPtr && !recvIsPtr {
						if pt := e.ptrTypeOf(base); pt != check.TypeInvalid {
							captureT = pt
							takeAddr = true
							recvIsPtr = true
						}
					}
					recvCT := e.cTypeFrom(captureT)
					fields = append([]deferField{{
						name:     "recv",
						ct:       recvCT,
						expr:     recvExpr,
						takeAddr: takeAddr,
					}}, fields...)
				}
			}
		}
	default:
		if check.IsFunc(e.typeOf(call.Fun)) {
			fnValue = true
			e.ensureFuncRuntime()
			fields = append([]deferField{{
				name: "fn",
				ct:   "uli_fn",
				expr: call.Fun,
			}}, fields...)
		}
	}

	// Thunk
	tb := &e.deferThunks
	fmt.Fprintf(tb, "static void %s(void *p) {\n", thunk)
	if len(fields) == 0 {
		tb.WriteString("\t(void)p;\n")
	} else {
		tb.WriteString("\tstruct { ")
		for _, f := range fields {
			fmt.Fprintf(tb, "%s %s; ", f.ct, f.name)
		}
		tb.WriteString("} *c = p;\n")
	}
	tb.WriteByte('\t')
	e.writeDeferredCall(tb, call, builtinName, localFunc, pkgFunc, method, methodStruct, recvIsPtr, fields, fnValue)
	tb.WriteString(";\n}\n")

	// Defer site: evaluate args, push frame.
	indent(b, level)
	b.WriteString("{\n")
	if len(fields) == 0 {
		indent(b, level+1)
		b.WriteString("uli_defer_frame *_df = (uli_defer_frame *)uli_arena_alloc(sizeof(uli_defer_frame));\n")
		indent(b, level+1)
		fmt.Fprintf(b, "_df->fn = %s;\n", thunk)
		indent(b, level+1)
		b.WriteString("_df->arg = NULL;\n")
		indent(b, level+1)
		b.WriteString("_df->next = _defers;\n")
		indent(b, level+1)
		b.WriteString("_defers = _df;\n")
	} else {
		indent(b, level+1)
		b.WriteString("struct { ")
		for _, f := range fields {
			fmt.Fprintf(b, "%s %s; ", f.ct, f.name)
		}
		b.WriteString("} *_dc = uli_arena_alloc(sizeof(*_dc));\n")
		for _, f := range fields {
			indent(b, level+1)
			fmt.Fprintf(b, "_dc->%s = ", f.name)
			if f.takeAddr {
				b.WriteByte('&')
			}
			e.writeExpr(b, f.expr)
			b.WriteString(";\n")
		}
		indent(b, level+1)
		b.WriteString("uli_defer_frame *_df = (uli_defer_frame *)uli_arena_alloc(sizeof(uli_defer_frame));\n")
		indent(b, level+1)
		fmt.Fprintf(b, "_df->fn = %s;\n", thunk)
		indent(b, level+1)
		b.WriteString("_df->arg = _dc;\n")
		indent(b, level+1)
		b.WriteString("_df->next = _defers;\n")
		indent(b, level+1)
		b.WriteString("_defers = _df;\n")
	}
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) writeDeferredCall(
	b *strings.Builder,
	call *ast.CallExpr,
	builtinName, localFunc, pkgFunc string,
	method *check.MethodInfo,
	methodStruct *check.StructInfo,
	recvIsPtr bool,
	fields []deferField,
	fnValue bool,
) {
	if fnValue {
		e.writeDeferredFnValue(b, call, fields)
		return
	}
	argFields := fields
	if method != nil && len(fields) > 0 && fields[0].name == "recv" {
		argFields = fields[1:]
	}
	fieldRef := func(i int) string {
		if i < 0 || i >= len(argFields) {
			return "0"
		}
		return "c->" + argFields[i].name
	}

	if builtinName != "" {
		switch builtinName {
		case "பதிப்பி":
			if len(argFields) != 1 {
				b.WriteString("(void)0")
				return
			}
			t := e.typeOf(call.Args[0])
			if e.info != nil {
				if si, ok := e.info.Structs[t]; ok {
					fmt.Fprintf(b, "%s(%s); printf(\"\\n\")", e.printFuncName(si), fieldRef(0))
					return
				}
				if under, ok := e.info.Underlying[t]; ok {
					t = under
				}
			}
			switch t {
			case check.TypeString:
				fmt.Fprintf(b, "uli_print_str(%s)", fieldRef(0))
			case check.TypeBool:
				fmt.Fprintf(b, "uli_print_bool(%s)", fieldRef(0))
			case check.TypeFloat:
				fmt.Fprintf(b, "uli_print_float(%s)", fieldRef(0))
			default:
				fmt.Fprintf(b, "uli_print_int(%s)", fieldRef(0))
			}
			return
		case "நீக்கு":
			if len(call.Args) >= 1 {
				mt := e.typeOf(call.Args[0])
				fmt.Fprintf(b, "%s(%s, %s)", e.mapDelName(mt), fieldRef(0), fieldRef(1))
				return
			}
		case "அலறு":
			e.needPanic = true
			if len(call.Args) == 0 || len(argFields) < 1 {
				b.WriteString("uli_panic(\"\")")
				return
			}
			e.writePanicCallRef(b, e.typeOf(call.Args[0]), fieldRef(0))
			return
		case "மீள்":
			e.needPanic = true
			b.WriteString("(void)uli_recover()")
			return
		case "நீளம்", "திறன்", "நகல்", "சேர்", "ஆக்கு":
			// Discard result; still invoke for side effects where relevant.
			b.WriteString("(void)(")
			e.writeDeferredBuiltin(b, builtinName, call, argFields)
			b.WriteByte(')')
			return
		}
		b.WriteString("(void)0")
		return
	}

	if method != nil && methodStruct != nil {
		b.WriteString(cPkgIdent(methodStruct.Pkg, methodStruct.Name+"_"+method.Name))
		b.WriteByte('(')
		if method.RecvIsPtr {
			if recvIsPtr {
				b.WriteString("c->recv")
			} else {
				b.WriteString("&c->recv")
			}
		} else {
			if recvIsPtr {
				b.WriteString("*c->recv")
			} else {
				b.WriteString("c->recv")
			}
		}
		for i := range argFields {
			b.WriteString(", ")
			b.WriteString(fieldRef(i))
		}
		b.WriteByte(')')
		return
	}

	if pkgFunc != "" {
		b.WriteString(pkgFunc)
	} else if localFunc != "" {
		b.WriteString(cPkgIdent(e.pkg, localFunc))
	} else {
		b.WriteString("/*bad defer*/(void)0")
		return
	}
	b.WriteByte('(')
	for i := range argFields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fieldRef(i))
	}
	b.WriteByte(')')
}

func (e *emitter) writeDeferredBuiltin(b *strings.Builder, name string, call *ast.CallExpr, fields []deferField) {
	fieldRef := func(i int) string {
		if i < 0 || i >= len(fields) {
			return "0"
		}
		return "c->" + fields[i].name
	}
	switch name {
	case "சேர்":
		if len(call.Args) == 0 {
			b.WriteString("0")
			return
		}
		st := e.typeOf(call.Args[0])
		fn := e.sliceAppendName(st)
		for i := 1; i < len(call.Args); i++ {
			b.WriteString(fn)
			b.WriteByte('(')
		}
		b.WriteString(fieldRef(0))
		for i := 1; i < len(call.Args); i++ {
			b.WriteString(", ")
			b.WriteString(fieldRef(i))
			b.WriteByte(')')
		}
	case "நகல்":
		st := e.typeOf(call.Args[0])
		fmt.Fprintf(b, "%s(%s, %s)", e.copyName(st), fieldRef(0), fieldRef(1))
	case "ஆக்கு":
		st := e.typeOf(call)
		if check.IsMap(st) {
			b.WriteString(e.mapMakeName(st))
			b.WriteByte('(')
			if len(call.Args) >= 1 {
				b.WriteString(fieldRef(0))
			} else {
				b.WriteString("0")
			}
			b.WriteByte(')')
			return
		}
		b.WriteString(e.makeName(st))
		b.WriteByte('(')
		b.WriteString(fieldRef(0))
		b.WriteString(", ")
		if len(call.Args) >= 2 {
			b.WriteString(fieldRef(1))
		} else {
			b.WriteString(fieldRef(0))
		}
		b.WriteByte(')')
	case "நீளம்":
		argT := e.typeOf(call.Args[0])
		if argT == check.TypeString {
			fmt.Fprintf(b, "((int64_t)strlen(%s))", fieldRef(0))
		} else if check.IsMap(argT) {
			fmt.Fprintf(b, "%s(%s)", e.mapLenName(argT), fieldRef(0))
		} else if check.IsArray(argT) {
			fmt.Fprintf(b, "%dLL", e.info.Arrays[argT].Len)
		} else {
			fmt.Fprintf(b, "(%s).len", fieldRef(0))
		}
	case "திறன்":
		argT := e.typeOf(call.Args[0])
		if check.IsArray(argT) {
			fmt.Fprintf(b, "%dLL", e.info.Arrays[argT].Len)
		} else {
			fmt.Fprintf(b, "(%s).cap", fieldRef(0))
		}
	default:
		b.WriteString("0")
	}
}

func usesSliceType(te ast.TypeExpr) bool {
	_, ok := te.(*ast.SliceType)
	return ok
}

func indent(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteByte('\t')
	}
}

func (e *emitter) writeBlock(b *strings.Builder, block *ast.BlockStmt, level int) {
	if block == nil {
		return
	}
	for _, s := range block.List {
		e.writeStmt(b, s, level)
	}
}

func (e *emitter) writeVarSpec(b *strings.Builder, spec *ast.VarSpec, level int) {
	if spec == nil {
		return
	}
	if spec.Type != nil && usesSliceType(spec.Type) {
		e.needSlice = true
	}
	for i, name := range spec.Names {
		var ct, zt string
		if spec.Type != nil {
			ct = e.cTypeExpr(spec.Type)
			zt = e.zeroInit(spec.Type)
		} else if i < len(spec.Values) {
			t := e.typeOf(spec.Values[i])
			ct = e.cTypeFrom(t)
			zt = e.zeroInitType(t)
		} else {
			ct, zt = "void", "0"
		}
		if e.isPromoted(name.Name) {
			t := e.promoted[name.Name]
			if i < len(spec.Values) {
				tmp := fmt.Sprintf("_ci_%d", e.swID)
				e.swID++
				indent(b, level)
				b.WriteString(ct)
				b.WriteByte(' ')
				b.WriteString(tmp)
				b.WriteString(" = ")
				e.writeExpr(b, spec.Values[i])
				b.WriteString(";\n")
				e.writePromoteAlloc(b, name.Name, t, tmp, level)
			} else {
				e.writePromoteAlloc(b, name.Name, t, zt, level)
			}
			continue
		}
		indent(b, level)
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(cIdent(name.Name))
		if i < len(spec.Values) {
			b.WriteString(" = ")
			e.writeExpr(b, spec.Values[i])
		} else {
			b.WriteString(" = ")
			b.WriteString(zt)
		}
		b.WriteString(";\n")
	}
}

func (e *emitter) writeShortVar(b *strings.Builder, s *ast.ShortVarDecl, level int) {
	if len(s.Names) > 1 && len(s.Values) == 1 {
		tmp := fmt.Sprintf("_mret_%d", e.swID)
		e.swID++
		rt := e.typeOf(s.Values[0])
		elems := e.info.TupleElems[rt]
		indent(b, level)
		b.WriteString(e.retCName(elems))
		b.WriteByte(' ')
		b.WriteString(tmp)
		b.WriteString(" = ")
		e.writeExpr(b, s.Values[0])
		b.WriteString(";\n")
		for i, name := range s.Names {
			if name.Name == "_" {
				continue
			}
			if e.isPromoted(name.Name) {
				t := e.promoted[name.Name]
				e.writePromoteAlloc(b, name.Name, t, fmt.Sprintf("%s.r%d", tmp, i), level)
				continue
			}
			indent(b, level)
			ct := e.cTypeFrom(elems[i])
			if strings.HasPrefix(ct, "uli_slice_") {
				e.needSlice = true
			}
			b.WriteString(ct)
			b.WriteByte(' ')
			b.WriteString(cIdent(name.Name))
			fmt.Fprintf(b, " = %s.r%d;\n", tmp, i)
		}
		return
	}
	for i, name := range s.Names {
		if name.Name == "_" {
			continue
		}
		if e.isPromoted(name.Name) {
			t := e.promoted[name.Name]
			// Evaluate init into a temp then store.
			tmp := fmt.Sprintf("_ci_%d", e.swID)
			e.swID++
			indent(b, level)
			b.WriteString(e.inferCType(s.Values[i]))
			b.WriteByte(' ')
			b.WriteString(tmp)
			b.WriteString(" = ")
			e.writeExpr(b, s.Values[i])
			b.WriteString(";\n")
			e.writePromoteAlloc(b, name.Name, t, tmp, level)
			continue
		}
		indent(b, level)
		ct := e.inferCType(s.Values[i])
		if strings.HasPrefix(ct, "uli_slice_") {
			e.needSlice = true
		}
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(cIdent(name.Name))
		b.WriteString(" = ")
		e.writeExpr(b, s.Values[i])
		b.WriteString(";\n")
	}
}

func (e *emitter) writeAssign(b *strings.Builder, s *ast.AssignStmt, level int) {
	if len(s.LHS) > 1 && len(s.Values) == 1 {
		tmp := fmt.Sprintf("_mret_%d", e.swID)
		e.swID++
		rt := e.typeOf(s.Values[0])
		elems := e.info.TupleElems[rt]
		indent(b, level)
		b.WriteString(e.retCName(elems))
		b.WriteByte(' ')
		b.WriteString(tmp)
		b.WriteString(" = ")
		e.writeExpr(b, s.Values[0])
		b.WriteString(";\n")
		for i, lhs := range s.LHS {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
				continue
			}
			indent(b, level)
			if idx, ok := lhs.(*ast.IndexExpr); ok {
				e.writeIndexSetName(b, idx, fmt.Sprintf("%s.r%d", tmp, i))
				b.WriteString(";\n")
				continue
			}
			e.writeExpr(b, lhs)
			fmt.Fprintf(b, " = %s.r%d;\n", tmp, i)
		}
		return
	}
	// Parallel assign with temps so a, b = b, a works.
	if len(s.LHS) > 1 && len(s.Values) == len(s.LHS) {
		type tmpSlot struct {
			name string
			typ  check.Type
		}
		tmps := make([]tmpSlot, len(s.Values))
		for i, v := range s.Values {
			tmps[i] = tmpSlot{
				name: fmt.Sprintf("_par_%d_%d", e.swID, i),
				typ:  e.typeOf(v),
			}
		}
		e.swID++
		for i, v := range s.Values {
			indent(b, level)
			b.WriteString(e.cTypeFrom(tmps[i].typ))
			b.WriteByte(' ')
			b.WriteString(tmps[i].name)
			b.WriteString(" = ")
			e.writeExpr(b, v)
			b.WriteString(";\n")
		}
		for i, lhs := range s.LHS {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
				continue
			}
			indent(b, level)
			if idx, ok := lhs.(*ast.IndexExpr); ok {
				e.writeIndexSetName(b, idx, tmps[i].name)
				b.WriteString(";\n")
				continue
			}
			e.writeExpr(b, lhs)
			b.WriteString(" = ")
			b.WriteString(tmps[i].name)
			b.WriteString(";\n")
		}
		return
	}
	indent(b, level)
	e.writeAssignInline(b, s)
	b.WriteString(";\n")
}

func (e *emitter) writeAssignInline(b *strings.Builder, s *ast.AssignStmt) {
	if len(s.LHS) != 1 || len(s.Values) != 1 {
		b.WriteString("/*bad assign*/0")
		return
	}
	if idx, ok := s.LHS[0].(*ast.IndexExpr); ok {
		e.writeIndexSet(b, idx, s.Values[0])
		return
	}
	e.writeExpr(b, s.LHS[0])
	b.WriteString(" = ")
	e.writeExpr(b, s.Values[0])
}

func (e *emitter) writeReturn(b *strings.Builder, s *ast.ReturnStmt, level int) {
	if e.fnHasUnwind() {
		e.writeReturnWithDefer(b, s, level)
		return
	}
	indent(b, level)
	b.WriteString("return")
	switch len(s.Results) {
	case 0:
		// Naked return: emit named result locals when present.
		if e.curFn != nil && len(e.curFn.Results) > 0 && e.curFn.Results[0].Name != nil {
			rts := e.resultTypes(e.curFn)
			b.WriteByte(' ')
			if len(rts) == 1 {
				b.WriteString(cIdent(e.curFn.Results[0].Name.Name))
			} else {
				fmt.Fprintf(b, "(%s){", e.retCName(rts))
				for i, r := range e.curFn.Results {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(cIdent(r.Name.Name))
				}
				b.WriteByte('}')
			}
		}
		b.WriteString(";\n")
	case 1:
		// Single expr: either single result or multi-value call returned as struct.
		b.WriteByte(' ')
		e.writeExpr(b, s.Results[0])
		b.WriteString(";\n")
	default:
		types := make([]check.Type, len(s.Results))
		for i, r := range s.Results {
			types[i] = e.typeOf(r)
		}
		b.WriteByte(' ')
		fmt.Fprintf(b, "(%s){", e.retCName(types))
		for i, r := range s.Results {
			if i > 0 {
				b.WriteString(", ")
			}
			e.writeExpr(b, r)
		}
		b.WriteString("};\n")
	}
}

func (e *emitter) writeReturnWithDefer(b *strings.Builder, s *ast.ReturnStmt, level int) {
	rts := e.resultTypes(e.curFn)
	named := e.fnNamedResults()
	switch len(s.Results) {
	case 0:
		// Naked / void: named locals already set.
	case 1:
		if named {
			if len(rts) == 1 {
				indent(b, level)
				b.WriteString(cIdent(e.curFn.Results[0].Name.Name))
				b.WriteString(" = ")
				e.writeExpr(b, s.Results[0])
				b.WriteString(";\n")
			} else {
				tmp := fmt.Sprintf("_mret_%d", e.swID)
				e.swID++
				indent(b, level)
				b.WriteString(e.retCName(rts))
				b.WriteByte(' ')
				b.WriteString(tmp)
				b.WriteString(" = ")
				e.writeExpr(b, s.Results[0])
				b.WriteString(";\n")
				for i, r := range e.curFn.Results {
					indent(b, level)
					b.WriteString(cIdent(r.Name.Name))
					fmt.Fprintf(b, " = %s.r%d;\n", tmp, i)
				}
			}
		} else {
			indent(b, level)
			b.WriteString("_ret = ")
			e.writeExpr(b, s.Results[0])
			b.WriteString(";\n")
		}
	default:
		if named {
			for i, r := range s.Results {
				if i >= len(e.curFn.Results) {
					break
				}
				indent(b, level)
				b.WriteString(cIdent(e.curFn.Results[i].Name.Name))
				b.WriteString(" = ")
				e.writeExpr(b, r)
				b.WriteString(";\n")
			}
		} else {
			types := make([]check.Type, len(s.Results))
			for i, r := range s.Results {
				types[i] = e.typeOf(r)
			}
			indent(b, level)
			b.WriteString("_ret = ")
			fmt.Fprintf(b, "(%s){", e.retCName(types))
			for i, r := range s.Results {
				if i > 0 {
					b.WriteString(", ")
				}
				e.writeExpr(b, r)
			}
			b.WriteString("};\n")
		}
	}
	indent(b, level)
	b.WriteString("goto __uli_epilogue;\n")
}

func (e *emitter) writeStmt(b *strings.Builder, s ast.Stmt, level int) {
	switch s := s.(type) {
	case *ast.ConstDecl:
		// Compile-time constants are inlined at use sites (Tamil-0.67).
		return
	case *ast.ConstGroupDecl:
		return
	case *ast.VarGroupDecl:
		for _, spec := range s.Specs {
			e.writeVarSpec(b, spec, level)
		}
	case *ast.VarDecl:
		e.writeVarSpec(b, &ast.VarSpec{Names: s.Names, Type: s.Type, Values: s.Values}, level)
	case *ast.ShortVarDecl:
		e.writeShortVar(b, s, level)
	case *ast.AssignStmt:
		e.writeAssign(b, s, level)
	case *ast.ExprStmt:
		indent(b, level)
		e.writeExpr(b, s.X)
		b.WriteString(";\n")
	case *ast.ReturnStmt:
		e.writeReturn(b, s, level)
	case *ast.DeferStmt:
		e.writeDefer(b, s, level)
	case *ast.GoStmt:
		e.writeGoStmt(b, s, level)
	case *ast.SendStmt:
		e.writeSend(b, s, level)
	case *ast.SelectStmt:
		e.writeSelect(b, s, level)
	case *ast.IfStmt:
		e.writeIf(b, s, level)
	case *ast.SwitchStmt:
		e.writeSwitch(b, s, level)
	case *ast.ForStmt:
		e.writeFor(b, s, level)
	case *ast.RangeStmt:
		e.writeRange(b, s, level)
	case *ast.BreakStmt:
		indent(b, level)
		b.WriteString("break;\n")
	case *ast.ContinueStmt:
		indent(b, level)
		b.WriteString("continue;\n")
	case *ast.BlockStmt:
		indent(b, level)
		b.WriteString("{\n")
		e.writeBlock(b, s, level+1)
		indent(b, level)
		b.WriteString("}\n")
	default:
		indent(b, level)
		b.WriteString("/* unsupported stmt */;\n")
	}
}

func (e *emitter) writeIf(b *strings.Builder, s *ast.IfStmt, level int) {
	if s.Init != nil {
		indent(b, level)
		b.WriteString("{\n")
		e.writeStmt(b, s.Init, level+1)
		e.writeIfChain(b, s, level+1)
		indent(b, level)
		b.WriteString("}\n")
		return
	}
	e.writeIfChain(b, s, level)
	b.WriteByte('\n')
}

// writeIfChain emits if/else without a trailing newline after the final '}'.
func (e *emitter) writeIfChain(b *strings.Builder, s *ast.IfStmt, level int) {
	indent(b, level)
	b.WriteString("if (")
	e.writeExpr(b, s.Cond)
	b.WriteString(") {\n")
	e.writeBlock(b, s.Body, level+1)
	indent(b, level)
	b.WriteString("}")
	if s.Else != nil {
		e.writeElseContinue(b, s.Else, level)
	}
}

func (e *emitter) writeSwitch(b *strings.Builder, s *ast.SwitchStmt, level int) {
	indent(b, level)
	b.WriteString("{\n")
	if s.Init != nil {
		e.writeStmt(b, s.Init, level+1)
	}
	tmp := ""
	tagT := check.TypeInvalid
	if s.Tag != nil {
		e.swID++
		tmp = fmt.Sprintf("__uli_sw_%d", e.swID)
		tagT = e.typeOf(s.Tag)
		indent(b, level+1)
		b.WriteString(e.cTypeFrom(tagT))
		b.WriteByte(' ')
		b.WriteString(tmp)
		b.WriteString(" = ")
		e.writeExpr(b, s.Tag)
		b.WriteString(";\n")
	}
	var def *ast.CaseClause
	first := true
	for _, cl := range s.Cases {
		if cl.Default {
			def = cl
			continue
		}
		indent(b, level+1)
		if first {
			b.WriteString("if (")
			first = false
		} else {
			b.WriteString("else if (")
		}
		e.writeCaseCond(b, tmp, tagT, cl)
		b.WriteString(") {\n")
		e.writeBlock(b, cl.Body, level+2)
		indent(b, level+1)
		b.WriteString("}\n")
	}
	if def != nil {
		indent(b, level+1)
		if first {
			// only default
			b.WriteString("{\n")
			e.writeBlock(b, def.Body, level+2)
			indent(b, level+1)
			b.WriteString("}\n")
		} else {
			b.WriteString("else {\n")
			e.writeBlock(b, def.Body, level+2)
			indent(b, level+1)
			b.WriteString("}\n")
		}
	}
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) writeCaseCond(b *strings.Builder, tmp string, tagT check.Type, cl *ast.CaseClause) {
	if tmp == "" {
		// tagless: OR of boolean exprs
		for i, x := range cl.List {
			if i > 0 {
				b.WriteString(" || ")
			}
			b.WriteByte('(')
			e.writeExpr(b, x)
			b.WriteByte(')')
		}
		return
	}
	for i, x := range cl.List {
		if i > 0 {
			b.WriteString(" || ")
		}
		if tagT == check.TypeString {
			b.WriteString("(strcmp(")
			b.WriteString(tmp)
			b.WriteString(", ")
			e.writeExpr(b, x)
			b.WriteString(") == 0)")
		} else {
			b.WriteByte('(')
			b.WriteString(tmp)
			b.WriteString(" == ")
			e.writeExpr(b, x)
			b.WriteByte(')')
		}
	}
}

func (e *emitter) writeFor(b *strings.Builder, s *ast.ForStmt, level int) {
	if s.Init == nil && s.Post == nil && s.Cond != nil {
		indent(b, level)
		b.WriteString("while (")
		e.writeExpr(b, s.Cond)
		b.WriteString(") {\n")
		e.writeGCPoll(b, level+1)
		e.writeBlock(b, s.Body, level+1)
		indent(b, level)
		b.WriteString("}\n")
		return
	}

	// Tamil-0.46: per-iteration vars for := init (Go 1.22+).
	sv, hasShort := s.Init.(*ast.ShortVarDecl)
	if hasShort && len(sv.Names) > 0 {
		e.writeForPerIter(b, s, sv, level)
		return
	}

	indent(b, level)
	b.WriteString("for (")
	e.writeForInit(b, s.Init)
	b.WriteString("; ")
	if s.Cond != nil {
		e.writeExpr(b, s.Cond)
	}
	b.WriteString("; ")
	e.writeForPost(b, s.Post)
	b.WriteString(") {\n")
	e.writeGCPoll(b, level+1)
	e.writeBlock(b, s.Body, level+1)
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) iterIdent(name string) string {
	return "__it_" + cIdent(name)
}

// writeForPerIter emits 3-clause for with Go 1.22 per-iteration body vars.
func (e *emitter) writeForPerIter(b *strings.Builder, s *ast.ForStmt, sv *ast.ShortVarDecl, level int) {
	indent(b, level)
	b.WriteString("{\n")
	for i, name := range sv.Names {
		if name.Name == "_" {
			continue
		}
		indent(b, level+1)
		ct := e.inferCType(sv.Values[i])
		if strings.HasPrefix(ct, "uli_slice_") {
			e.needSlice = true
		}
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(e.iterIdent(name.Name))
		b.WriteString(" = ")
		e.writeExpr(b, sv.Values[i])
		b.WriteString(";\n")
	}

	e.loopIter = map[string]string{}
	for _, name := range sv.Names {
		if name.Name != "_" {
			e.loopIter[name.Name] = e.iterIdent(name.Name)
		}
	}
	indent(b, level+1)
	b.WriteString("for (; ")
	if s.Cond != nil {
		e.writeExpr(b, s.Cond)
	}
	b.WriteString("; ")
	e.writeForPost(b, s.Post)
	b.WriteString(") {\n")
	e.writeGCPoll(b, level+2)
	e.loopIter = nil

	for i, name := range sv.Names {
		if name.Name == "_" {
			continue
		}
		it := e.iterIdent(name.Name)
		if e.isPromoted(name.Name) {
			t := e.promoted[name.Name]
			e.writePromoteAlloc(b, name.Name, t, it, level+2)
			continue
		}
		indent(b, level+2)
		ct := e.inferCType(sv.Values[i])
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(cIdent(name.Name))
		b.WriteString(" = ")
		b.WriteString(it)
		b.WriteString(";\n")
	}

	e.writeBlock(b, s.Body, level+2)
	indent(b, level+1)
	b.WriteString("}\n")
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) writeRange(b *strings.Builder, s *ast.RangeStmt, level int) {
	xt := e.typeOf(s.X)
	if xt == check.TypeString {
		e.writeRangeString(b, s, level)
		return
	}
	if check.IsMap(xt) {
		e.writeRangeMap(b, s, level)
		return
	}
	if check.IsChan(xt) {
		e.writeRangeChan(b, s, level)
		return
	}
	isArr := check.IsArray(xt)
	if !isArr {
		e.needSlice = true
	}
	elemT := e.elemType(xt)
	cursor := "_ri"

	indent(b, level)
	b.WriteString("{\n")
	indent(b, level+1)
	b.WriteString("for (int64_t ")
	b.WriteString(cursor)
	b.WriteString(" = 0; ")
	b.WriteString(cursor)
	b.WriteString(" < ")
	if isArr {
		fmt.Fprintf(b, "%dLL", e.info.Arrays[xt].Len)
	} else {
		b.WriteByte('(')
		e.writeExpr(b, s.X)
		b.WriteString(").len")
	}
	b.WriteString("; ")
	b.WriteString(cursor)
	b.WriteString("++) {\n")
	e.writeGCPoll(b, level+2)

	if s.Key != nil && s.Key.Name != "_" {
		e.writeRangeIterBind(b, s.Define, s.Key.Name, check.TypeInt, "int64_t", cursor, level+2)
	}
	if s.Value != nil && s.Value.Name != "_" {
		var elemInit strings.Builder
		e.writeRangeElem(&elemInit, s.X, xt, cursor)
		e.writeRangeIterBind(b, s.Define, s.Value.Name, elemT, e.cTypeFrom(elemT), elemInit.String(), level+2)
	}

	e.writeBlock(b, s.Body, level+2)
	indent(b, level+1)
	b.WriteString("}\n")
	indent(b, level)
	b.WriteString("}\n")
}

// writeRangeIterBind binds a range key/value for one iteration (Go 1.22+ when Define).
func (e *emitter) writeRangeIterBind(b *strings.Builder, define bool, name string, t check.Type, ct, initExpr string, level int) {
	if define {
		if e.isPromoted(name) {
			e.writePromoteAlloc(b, name, t, initExpr, level)
			return
		}
		indent(b, level)
		b.WriteString(ct)
		b.WriteByte(' ')
		b.WriteString(cIdent(name))
		b.WriteString(" = ")
		b.WriteString(initExpr)
		b.WriteString(";\n")
		return
	}
	// Assignment form: update outer variable.
	indent(b, level)
	if e.isPromoted(name) {
		e.writePromotedIdent(b, name)
	} else {
		b.WriteString(cIdent(name))
	}
	b.WriteString(" = ")
	b.WriteString(initExpr)
	b.WriteString(";\n")
}

func (e *emitter) writeRangeChan(b *strings.Builder, s *ast.RangeStmt, level int) {
	e.needChan = true
	xt := e.typeOf(s.X)
	ci := e.info.Chans[xt]
	elemCT := e.cTypeFrom(ci.Elem)
	id := e.goID
	e.goID++
	ch := fmt.Sprintf("__rch_%d", id)
	val := fmt.Sprintf("__rv_%d", id)
	ok := fmt.Sprintf("__rok_%d", id)

	indent(b, level)
	b.WriteString("{\n")
	indent(b, level+1)
	b.WriteString("uli_chan *")
	b.WriteString(ch)
	b.WriteString(" = ")
	e.writeExpr(b, s.X)
	b.WriteString(";\n")
	indent(b, level+1)
	b.WriteString("for (;;) {\n")
	e.writeGCPoll(b, level+2)
	indent(b, level+2)
	b.WriteString(elemCT)
	b.WriteByte(' ')
	b.WriteString(val)
	b.WriteString(";\n")
	indent(b, level+2)
	fmt.Fprintf(b, "int %s = uli_chan_recv(%s, &%s);\n", ok, ch, val)
	indent(b, level+2)
	fmt.Fprintf(b, "if (!%s) break;\n", ok)
	if s.Key != nil && s.Key.Name != "_" {
		e.writeRangeIterBind(b, s.Define, s.Key.Name, ci.Elem, elemCT, val, level+2)
	}
	e.writeBlock(b, s.Body, level+2)
	indent(b, level+1)
	b.WriteString("}\n")
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) writeRangeMap(b *strings.Builder, s *ast.RangeStmt, level int) {
	e.needMap = true
	xt := e.typeOf(s.X)
	mi := e.info.Maps[xt]
	indent(b, level)
	b.WriteString("{\n")
	indent(b, level+1)
	b.WriteString(e.mapCName(xt))
	b.WriteString(" _rm = ")
	e.writeExpr(b, s.X)
	b.WriteString(";\n")
	indent(b, level+1)
	b.WriteString("if (_rm) for (int64_t _ri = 0; _ri < _rm->cap; _ri++) {\n")
	e.writeGCPoll(b, level+2)
	indent(b, level+2)
	b.WriteString("if (_rm->entries[_ri].state != 1) continue;\n")
	if s.Key != nil && s.Key.Name != "_" {
		e.writeRangeIterBind(b, s.Define, s.Key.Name, mi.Key, e.cTypeFrom(mi.Key), "_rm->entries[_ri].key", level+2)
	}
	if s.Value != nil && s.Value.Name != "_" {
		e.writeRangeIterBind(b, s.Define, s.Value.Name, mi.Elem, e.cTypeFrom(mi.Elem), "_rm->entries[_ri].val", level+2)
	}
	e.writeBlock(b, s.Body, level+2)
	indent(b, level+1)
	b.WriteString("}\n")
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) writeRangeString(b *strings.Builder, s *ast.RangeStmt, level int) {
	e.needUTF8 = true
	indent(b, level)
	b.WriteString("{\n")
	indent(b, level+1)
	b.WriteString("const char *_rs = ")
	e.writeExpr(b, s.X)
	b.WriteString(";\n")
	indent(b, level+1)
	b.WriteString("int64_t _rcur = 0;\n")
	indent(b, level+1)
	b.WriteString("while (_rs[_rcur]) {\n")
	e.writeGCPoll(b, level+2)
	if s.Key != nil && s.Key.Name != "_" {
		e.writeRangeIterBind(b, s.Define, s.Key.Name, check.TypeInt, "int64_t", "_rcur", level+2)
	}
	indent(b, level+2)
	if s.Value != nil && s.Value.Name != "_" {
		if s.Define && e.isPromoted(s.Value.Name) {
			tmp := fmt.Sprintf("_rv_%d", e.swID)
			e.swID++
			b.WriteString("int64_t ")
			b.WriteString(tmp)
			b.WriteString(" = uli_utf8_next(_rs, &_rcur);\n")
			e.writePromoteAlloc(b, s.Value.Name, check.TypeInt, tmp, level+2)
		} else if s.Define {
			b.WriteString("int64_t ")
			b.WriteString(cIdent(s.Value.Name))
			b.WriteString(" = uli_utf8_next(_rs, &_rcur);\n")
		} else {
			if e.isPromoted(s.Value.Name) {
				e.writePromotedIdent(b, s.Value.Name)
			} else {
				b.WriteString(cIdent(s.Value.Name))
			}
			b.WriteString(" = uli_utf8_next(_rs, &_rcur);\n")
		}
	} else {
		b.WriteString("(void)uli_utf8_next(_rs, &_rcur);\n")
	}
	e.writeBlock(b, s.Body, level+2)
	indent(b, level+1)
	b.WriteString("}\n")
	indent(b, level)
	b.WriteString("}\n")
}

func (e *emitter) elemType(t check.Type) check.Type {
	if check.IsArray(t) {
		return e.info.Arrays[t].Elem
	}
	el := e.sliceElem(t)
	if el == check.TypeInvalid {
		return check.TypeInt
	}
	return el
}

func (e *emitter) writeRangeElem(b *strings.Builder, x ast.Expr, xt check.Type, idxName string) {
	if check.IsArray(xt) {
		b.WriteString(e.arrayCName(xt))
		b.WriteString("_get(")
		e.writeExpr(b, x)
		b.WriteString(", ")
		b.WriteString(idxName)
		b.WriteByte(')')
		return
	}
	b.WriteString(e.sliceGetName(xt))
	b.WriteByte('(')
	e.writeExpr(b, x)
	b.WriteString(", ")
	b.WriteString(idxName)
	b.WriteByte(')')
}

func (e *emitter) writeForInit(b *strings.Builder, s ast.Stmt) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *ast.ShortVarDecl:
		for i, name := range s.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(e.inferCType(s.Values[i]))
			b.WriteByte(' ')
			b.WriteString(cIdent(name.Name))
			b.WriteString(" = ")
			e.writeExpr(b, s.Values[i])
		}
	case *ast.AssignStmt:
		e.writeAssignInline(b, s)
	case *ast.ExprStmt:
		e.writeExpr(b, s.X)
	case *ast.VarDecl:
		for i, name := range s.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(e.cTypeExpr(s.Type))
			b.WriteByte(' ')
			b.WriteString(cIdent(name.Name))
			if i < len(s.Values) {
				b.WriteString(" = ")
				e.writeExpr(b, s.Values[i])
			} else {
				b.WriteString(" = ")
				b.WriteString(e.zeroInit(s.Type))
			}
		}
	}
}

func (e *emitter) writeForPost(b *strings.Builder, s ast.Stmt) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *ast.AssignStmt:
		e.writeAssignInline(b, s)
	case *ast.ExprStmt:
		e.writeExpr(b, s.X)
	case *ast.ShortVarDecl:
		if len(s.Names) > 0 && len(s.Values) > 0 {
			b.WriteString(cIdent(s.Names[0].Name))
			b.WriteString(" = ")
			e.writeExpr(b, s.Values[0])
		}
	}
}

func (e *emitter) writeElseContinue(b *strings.Builder, elseStmt ast.Stmt, level int) {
	switch el := elseStmt.(type) {
	case *ast.BlockStmt:
		b.WriteString(" else {\n")
		e.writeBlock(b, el, level+1)
		indent(b, level)
		b.WriteString("}")
	case *ast.IfStmt:
		if el.Init != nil {
			b.WriteString(" else {\n")
			e.writeStmt(b, el.Init, level+1)
			e.writeIfChain(b, el, level+1)
			b.WriteByte('\n')
			indent(b, level)
			b.WriteString("}")
			return
		}
		b.WriteString(" else if (")
		e.writeExpr(b, el.Cond)
		b.WriteString(") {\n")
		e.writeBlock(b, el.Body, level+1)
		indent(b, level)
		b.WriteString("}")
		if el.Else != nil {
			e.writeElseContinue(b, el.Else, level)
		}
	}
}

func (e *emitter) inferCType(expr ast.Expr) string {
	if t, ok := e.info.Types[expr]; ok && t != check.TypeInvalid && t != check.TypeVoid {
		return e.cTypeFrom(t)
	}
	switch expr := expr.(type) {
	case *ast.BoolLit:
		return "int"
	case *ast.BasicLit:
		switch expr.Kind {
		case token.STRING:
			return "const char *"
		case token.FLOAT:
			return "double"
		default:
			return "int64_t"
		}
	case *ast.CompositeLit:
		e.needSlice = true
		return e.cTypeExpr(expr.Type)
	case *ast.UnaryExpr:
		switch expr.Op {
		case token.NOT:
			return "int"
		case token.AND, token.MUL:
			if t, ok := e.info.Types[expr]; ok && t != check.TypeInvalid {
				return e.cTypeFrom(t)
			}
			fallthrough
		default:
			return "int64_t"
		}
	case *ast.BinaryExpr:
		switch expr.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return "int"
		case token.ADD:
			if e.typeOf(expr.X) == check.TypeString {
				return "const char *"
			}
			return "int64_t"
		default:
			return "int64_t"
		}
	default:
		return "int64_t"
	}
}

func (e *emitter) typeOf(expr ast.Expr) check.Type {
	if e.info == nil || expr == nil {
		return check.TypeInvalid
	}
	if t, ok := e.info.Types[expr]; ok {
		return e.concreteType(t)
	}
	return check.TypeInvalid
}

// concreteType applies the current monomorphization substitution to a type
// recorded during schematic checking of a generic function body.
func (e *emitter) concreteType(t check.Type) check.Type {
	if e.typeSubst == nil || e.info == nil || t == check.TypeInvalid {
		return t
	}
	if name, ok := e.info.TypeParamName[t]; ok {
		if ct, ok := e.typeSubst[name]; ok {
			return ct
		}
		return t
	}
	if check.IsSlice(t) {
		elem := e.concreteType(check.ElemOfSlice(e.info, t))
		if st := e.findSliceOf(elem); st != check.TypeInvalid {
			return st
		}
		return t
	}
	if elem, ok := e.info.PtrElem[t]; ok {
		ce := e.concreteType(elem)
		for pt, el := range e.info.PtrElem {
			if el == ce {
				return pt
			}
		}
		return t
	}
	if ai, ok := e.info.Arrays[t]; ok {
		ce := e.concreteType(ai.Elem)
		for at, a := range e.info.Arrays {
			if a.Len == ai.Len && a.Elem == ce {
				return at
			}
		}
		return t
	}
	if mi, ok := e.info.Maps[t]; ok {
		ck := e.concreteType(mi.Key)
		ce := e.concreteType(mi.Elem)
		for mt, m := range e.info.Maps {
			if m.Key == ck && m.Elem == ce {
				return mt
			}
		}
		return t
	}
	return t
}

func (e *emitter) writeConstValue(b *strings.Builder, v check.ConstValue) {
	switch v.Kind {
	case check.ConstInt:
		b.WriteString(strconv.FormatInt(v.Int, 10))
		b.WriteString("LL")
	case check.ConstFloat:
		b.WriteString(strconv.FormatFloat(v.Float, 'g', -1, 64))
	case check.ConstBool:
		if v.Bool {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	case check.ConstString:
		b.WriteByte('"')
		b.WriteString(escapeCString(v.Str))
		b.WriteByte('"')
	default:
		b.WriteString("0")
	}
}

func (e *emitter) writeExpr(b *strings.Builder, expr ast.Expr) {
	if e.info != nil && e.info.ConstExprs != nil {
		if v, ok := e.info.ConstExprs[expr]; ok && v.Kind != check.ConstInvalid {
			e.writeConstValue(b, v)
			return
		}
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		if e.loopIter != nil {
			if it, ok := e.loopIter[expr.Name]; ok {
				b.WriteString(it)
				return
			}
		}
		if e.info != nil {
			if vv := e.info.PkgVarValues[expr]; vv != nil {
				b.WriteString(cPkgIdent(vv.Pkg, vv.Name))
				return
			}
			if fv := e.info.PkgFuncValues[expr]; fv != nil {
				e.writePkgFuncValue(b, fv)
				return
			}
		}
		if e.isPromoted(expr.Name) {
			e.writePromotedIdent(b, expr.Name)
			return
		}
		b.WriteString(cIdent(expr.Name))
	case *ast.BasicLit:
		switch expr.Kind {
		case token.STRING:
			b.WriteByte('"')
			b.WriteString(escapeCString(expr.Value))
			b.WriteByte('"')
		case token.FLOAT:
			b.WriteString(expr.Value)
		default:
			b.WriteString(expr.Value)
			b.WriteString("LL")
		}
	case *ast.BoolLit:
		if expr.Value {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	case *ast.NilLit:
		b.WriteString("NULL")
	case *ast.ParenExpr:
		b.WriteByte('(')
		e.writeExpr(b, expr.X)
		b.WriteByte(')')
	case *ast.UnaryExpr:
		if expr.Op == token.AND {
			b.WriteByte('(')
			e.writeAddrOf(b, expr.X)
			b.WriteByte(')')
			return
		}
		if expr.Op == token.ARROW {
			e.writeRecvExpr(b, expr)
			return
		}
		b.WriteByte('(')
		switch expr.Op {
		case token.SUB:
			b.WriteByte('-')
		case token.NOT:
			b.WriteByte('!')
		case token.MUL:
			b.WriteByte('*')
		case token.XOR:
			b.WriteByte('~')
		}
		e.writeExpr(b, expr.X)
		b.WriteByte(')')
	case *ast.BinaryExpr:
		e.writeBinary(b, expr)
	case *ast.CallExpr:
		if expr.Conversion {
			e.writeConversion(b, expr)
			return
		}
		if expr.Builtin {
			id, _ := expr.Fun.(*ast.Ident)
			if id != nil && id.Name == "நீளம்" {
				argT := e.typeOf(expr.Args[0])
				if argT == check.TypeString {
					b.WriteString("((int64_t)strlen(")
					e.writeExpr(b, expr.Args[0])
					b.WriteString("))")
					return
				}
				if check.IsArray(argT) {
					fmt.Fprintf(b, "%dLL", e.info.Arrays[argT].Len)
					return
				}
				if check.IsMap(argT) {
					e.needMap = true
					b.WriteString(e.mapLenName(argT))
					b.WriteByte('(')
					e.writeExpr(b, expr.Args[0])
					b.WriteByte(')')
					return
				}
				e.needSlice = true
				b.WriteByte('(')
				e.writeExpr(b, expr.Args[0])
				b.WriteString(").len")
				return
			}
			if id != nil && id.Name == "நீக்கு" {
				mt := e.typeOf(expr.Args[0])
				e.needMap = true
				b.WriteString(e.mapDelName(mt))
				b.WriteByte('(')
				e.writeExpr(b, expr.Args[0])
				b.WriteString(", ")
				e.writeExpr(b, expr.Args[1])
				b.WriteByte(')')
				return
			}
			if id != nil && id.Name == "மூடு" {
				e.needChan = true
				b.WriteString("uli_chan_close(")
				e.writeExpr(b, expr.Args[0])
				b.WriteByte(')')
				return
			}
			if id != nil && id.Name == "அலறு" {
				e.needPanic = true
				if len(expr.Args) > 0 {
					e.writePanicCall(b, expr.Args[0])
				} else {
					b.WriteString("uli_panic(\"\")")
				}
				return
			}
			if id != nil && id.Name == "மீள்" {
				e.needPanic = true
				b.WriteString("uli_recover()")
				return
			}
			if id != nil && id.Name == "சேர்" {
				e.needAppend = true
				e.needSlice = true
				e.writeAppend(b, expr)
				return
			}
			if id != nil && id.Name == "திறன்" {
				argT := e.typeOf(expr.Args[0])
				if check.IsArray(argT) {
					fmt.Fprintf(b, "%dLL", e.info.Arrays[argT].Len)
					return
				}
				e.needSlice = true
				b.WriteByte('(')
				e.writeExpr(b, expr.Args[0])
				b.WriteString(").cap")
				return
			}
			if id != nil && id.Name == "நகல்" {
				e.needCopy = true
				e.needSlice = true
				e.writeCopy(b, expr)
				return
			}
			if id != nil && id.Name == "ஆக்கு" {
				st := e.typeOf(expr)
				if check.IsMap(st) {
					e.needMap = true
					e.needArena = true
					e.writeMake(b, expr)
					return
				}
				e.needMake = true
				e.needSlice = true
				e.needArena = true
				e.writeMake(b, expr)
				return
			}
			e.writePrint(b, expr.Args[0])
			return
		}
		if e.info != nil {
			if inst := e.info.CallInst[expr]; inst != nil {
				b.WriteString(e.monoCName(inst))
				b.WriteByte('(')
				e.writeCallArgs(b, expr)
				b.WriteByte(')')
				return
			}
		}
		if ft := e.typeOf(expr.Fun); check.IsFunc(ft) {
			e.writeFuncValueCall(b, expr, ft)
			return
		}
		if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
			if e.info != nil && e.info.FmtPlans[expr] != nil {
				e.writeFmtSprintfCall(b, expr)
				return
			}
			if e.typeOf(sel.X) == check.TypeInvalid {
				if id, ok := sel.X.(*ast.Ident); ok {
					b.WriteString(cPkgIdent(e.realPkg(id.Name), sel.Sel.Name))
					b.WriteByte('(')
					e.writeCallArgs(b, expr)
					b.WriteByte(')')
					return
				}
			}
			e.writeMethodCall(b, expr, sel)
			return
		}
		if _, ok := expr.Fun.(*ast.IndexExpr); ok {
			b.WriteString("/*bad generic call*/0")
			return
		}
		id, ok := expr.Fun.(*ast.Ident)
		if !ok {
			b.WriteString("/*bad call*/0")
			return
		}
		b.WriteString(cPkgIdent(e.pkg, id.Name))
		b.WriteByte('(')
		e.writeCallArgs(b, expr)
		b.WriteByte(')')
	case *ast.CompositeLit:
		e.writeCompositeLit(b, expr)
	case *ast.IndexExpr:
		e.writeIndexGet(b, expr)
	case *ast.SliceExpr:
		e.writeSliceExpr(b, expr)
	case *ast.SelectorExpr:
		if e.info != nil {
			if vv := e.info.PkgVarValues[expr]; vv != nil {
				b.WriteString(cPkgIdent(vv.Pkg, vv.Name))
				return
			}
			if me := e.info.MethodExprs[expr]; me != nil {
				e.writeMethodExpr(b, me)
				return
			}
			if mv := e.info.MethodValues[expr]; mv != nil {
				e.writeMethodValue(b, expr, mv)
				return
			}
			if fv := e.info.PkgFuncValues[expr]; fv != nil {
				e.writePkgFuncValue(b, fv)
				return
			}
		}
		e.writeExpr(b, expr.X)
		if e.info != nil {
			if _, ok := e.info.PtrElem[e.typeOf(expr.X)]; ok {
				b.WriteString("->")
			} else {
				b.WriteByte('.')
			}
		} else {
			b.WriteByte('.')
		}
		b.WriteString(cIdent(expr.Sel.Name))
	case *ast.FuncLit:
		e.writeFuncLit(b, expr)
	default:
		b.WriteString("/*expr*/0")
	}
}

func (e *emitter) peelUnderlying(t check.Type) check.Type {
	if e.info != nil {
		if u, ok := e.info.Underlying[t]; ok {
			return u
		}
	}
	return t
}

func (e *emitter) writeConversion(b *strings.Builder, call *ast.CallExpr) {
	dest := e.typeOf(call)
	if len(call.Args) == 1 {
		src := e.peelUnderlying(e.typeOf(call.Args[0]))
		du := e.peelUnderlying(dest)
		if du == check.TypeString && (src == check.TypeInt || src == check.TypeRune || src == check.TypeByte) {
			e.needRuneStr = true
			e.needArena = true
			b.WriteString("uli_rune_str((int64_t)(")
			e.writeExpr(b, call.Args[0])
			b.WriteString("))")
			return
		}
		if du == check.TypeString && src == check.TypeSliceByte {
			e.needStrBytes = true
			e.needArena = true
			b.WriteString("uli_bytes_str(")
			e.writeExpr(b, call.Args[0])
			b.WriteByte(')')
			return
		}
		if du == check.TypeString && src == check.TypeSliceRune {
			e.needStrBytes = true
			e.needArena = true
			b.WriteString("uli_runes_str(")
			e.writeExpr(b, call.Args[0])
			b.WriteByte(')')
			return
		}
		if du == check.TypeSliceByte && src == check.TypeString {
			e.needStrBytes = true
			e.needArena = true
			b.WriteString("uli_str_bytes(")
			e.writeExpr(b, call.Args[0])
			b.WriteByte(')')
			return
		}
		if du == check.TypeSliceRune && src == check.TypeString {
			e.needStrBytes = true
			e.needArena = true
			b.WriteString("uli_str_runes(")
			e.writeExpr(b, call.Args[0])
			b.WriteByte(')')
			return
		}
	}
	b.WriteByte('(')
	b.WriteString(e.cTypeFrom(dest))
	b.WriteString(")(")
	if len(call.Args) == 1 {
		e.writeExpr(b, call.Args[0])
	} else {
		b.WriteByte('0')
	}
	b.WriteByte(')')
}

func (e *emitter) writeMethodCall(b *strings.Builder, call *ast.CallExpr, sel *ast.SelectorExpr) {
	xt := e.typeOf(sel.X)
	base := xt
	if elem, ok := e.info.PtrElem[xt]; ok {
		base = elem
	}
	si := e.info.Structs[base]
	mi := si.Methods[sel.Sel.Name]
	b.WriteString(cPkgIdent(si.Pkg, si.Name+"_"+mi.Name))
	b.WriteByte('(')
	if mi.RecvIsPtr {
		if _, isPtr := e.info.PtrElem[xt]; isPtr {
			e.writeExpr(b, sel.X)
		} else {
			e.writeAddrOf(b, sel.X)
		}
	} else {
		if _, isPtr := e.info.PtrElem[xt]; isPtr {
			b.WriteString("*")
			e.writeExpr(b, sel.X)
		} else {
			e.writeExpr(b, sel.X)
		}
	}
	if len(call.Args) > 0 || (e.info != nil && e.info.VariadicPacks[call] != nil) {
		b.WriteString(", ")
		e.writeCallArgs(b, call)
	}
	b.WriteByte(')')
}

func (e *emitter) writeMake(b *strings.Builder, call *ast.CallExpr) {
	st := e.typeOf(call)
	if !check.IsSlice(st) && !check.IsMap(st) && !check.IsChan(st) && call.TypeArg != nil {
		st = e.resolveTypeExpr(call.TypeArg)
	}
	if check.IsMap(st) {
		e.needMap = true
		e.needArena = true
		b.WriteString(e.mapMakeName(st))
		b.WriteByte('(')
		if len(call.Args) >= 1 {
			e.writeExpr(b, call.Args[0])
		} else {
			b.WriteString("0")
		}
		b.WriteByte(')')
		return
	}
	if check.IsChan(st) {
		e.needChan = true
		ci := e.info.Chans[st]
		fmt.Fprintf(b, "uli_chan_make((int)sizeof(%s), ", e.cTypeFrom(ci.Elem))
		if len(call.Args) >= 1 {
			e.writeExpr(b, call.Args[0])
		} else {
			b.WriteString("0")
		}
		b.WriteByte(')')
		return
	}
	b.WriteString(e.makeName(st))
	b.WriteByte('(')
	e.writeExpr(b, call.Args[0])
	b.WriteString(", ")
	if len(call.Args) >= 2 {
		e.writeExpr(b, call.Args[1])
	} else {
		e.writeExpr(b, call.Args[0])
	}
	b.WriteByte(')')
}

func (e *emitter) writeCopy(b *strings.Builder, call *ast.CallExpr) {
	st := e.typeOf(call.Args[0])
	b.WriteString(e.copyName(st))
	b.WriteByte('(')
	e.writeExpr(b, call.Args[0])
	b.WriteString(", ")
	e.writeExpr(b, call.Args[1])
	b.WriteByte(')')
}

func (e *emitter) writeAppend(b *strings.Builder, call *ast.CallExpr) {
	st := e.typeOf(call.Args[0])
	fn := e.sliceAppendName(st)
	// Nest: சேர்(xs, a, b) → append(append(xs, a), b)
	for i := 1; i < len(call.Args); i++ {
		b.WriteString(fn)
		b.WriteByte('(')
	}
	e.writeExpr(b, call.Args[0])
	for i := 1; i < len(call.Args); i++ {
		b.WriteString(", ")
		e.writeExpr(b, call.Args[i])
		b.WriteByte(')')
	}
}

func (e *emitter) writeSliceExpr(b *strings.Builder, s *ast.SliceExpr) {
	e.needSlice = true
	xt := e.typeOf(s.X)
	if xt == check.TypeString {
		e.needArena = true
		b.WriteString("uli_sub_string(")
		e.writeExpr(b, s.X)
		b.WriteString(", ")
		e.writeBound(b, s.Low, "0")
		b.WriteString(", ")
		if s.High != nil {
			e.writeExpr(b, s.High)
		} else {
			b.WriteString("((int64_t)strlen(")
			e.writeExpr(b, s.X)
			b.WriteString("))")
		}
		b.WriteByte(')')
		return
	}
	if check.IsArray(xt) {
		ai := e.info.Arrays[xt]
		name := e.arrayCName(xt)
		b.WriteString(name)
		b.WriteString("_slice(&(")
		e.writeExpr(b, s.X)
		b.WriteString("), ")
		e.writeBound(b, s.Low, "0")
		b.WriteString(", ")
		if s.High != nil {
			e.writeExpr(b, s.High)
		} else {
			fmt.Fprintf(b, "%dLL", ai.Len)
		}
		b.WriteString(", ")
		if s.Max != nil {
			e.writeExpr(b, s.Max)
		} else {
			fmt.Fprintf(b, "%dLL", ai.Len)
		}
		b.WriteByte(')')
		return
	}
	if s.Slice3 || s.Max != nil {
		b.WriteString(e.sliceSub3Name(xt))
		b.WriteByte('(')
		e.writeExpr(b, s.X)
		b.WriteString(", ")
		e.writeBound(b, s.Low, "0")
		b.WriteString(", ")
		e.writeExpr(b, s.High)
		b.WriteString(", ")
		e.writeExpr(b, s.Max)
		b.WriteByte(')')
		return
	}
	b.WriteString(e.sliceSubName(xt))
	b.WriteByte('(')
	e.writeExpr(b, s.X)
	b.WriteString(", ")
	e.writeBound(b, s.Low, "0")
	b.WriteString(", ")
	e.writeBoundHigh(b, s)
	b.WriteByte(')')
}

func (e *emitter) writeBound(b *strings.Builder, expr ast.Expr, zero string) {
	if expr == nil {
		b.WriteString(zero)
		return
	}
	e.writeExpr(b, expr)
}

func (e *emitter) writeBoundHigh(b *strings.Builder, s *ast.SliceExpr) {
	if s.High != nil {
		e.writeExpr(b, s.High)
		return
	}
	b.WriteByte('(')
	e.writeExpr(b, s.X)
	b.WriteString(").len")
}

func (e *emitter) writeCompositeLit(b *strings.Builder, lit *ast.CompositeLit) {
	if tn, ok := lit.Type.(*ast.TypeName); ok {
		if t, ok := e.lookupNamed(tn); ok {
			if si, ok := e.info.Structs[t]; ok {
				b.WriteString("(")
				b.WriteString(cPkgIdent(si.Pkg, si.Name))
				b.WriteString("){")
				for i, el := range lit.Elts {
					if i > 0 {
						b.WriteString(", ")
					}
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						b.WriteByte('.')
						if id, ok := kv.Key.(*ast.Ident); ok {
							b.WriteString(cIdent(id.Name))
						} else {
							b.WriteString("/*bad*/")
						}
						b.WriteString(" = ")
						e.writeExpr(b, kv.Value)
						continue
					}
					if i >= len(si.Fields) {
						b.WriteString("/*bad*/0")
						continue
					}
					b.WriteByte('.')
					b.WriteString(cIdent(si.Fields[i].Name))
					b.WriteString(" = ")
					e.writeExpr(b, el)
				}
				b.WriteByte('}')
				return
			}
		}
	}
	litT := e.typeOf(lit)
	if litT == check.TypeInvalid {
		litT = e.resolveTypeExpr(lit.Type)
	}
	if check.IsMap(litT) {
		e.writeMapLit(b, lit, litT)
		return
	}
	if check.IsArray(litT) {
		ai := e.info.Arrays[litT]
		fmt.Fprintf(b, "(%s){ .data = {", e.arrayCName(litT))
		for i, el := range lit.Elts {
			if i > 0 {
				b.WriteString(", ")
			}
			e.writeExpr(b, el)
		}
		if len(lit.Elts) == 0 {
			b.WriteString(e.zeroCValue(ai.Elem))
		}
		b.WriteString("} }")
		return
	}
	e.needSlice = true
	if !check.IsSlice(litT) {
		b.WriteString("/*bad lit*/{0}")
		return
	}
	sliceTy := e.sliceCName(litT)
	elemT := e.sliceElem(litT)
	arrTy := e.cTypeFrom(elemT)
	fmt.Fprintf(b, "(%s){ (%s[]){", sliceTy, arrTy)
	for i, el := range lit.Elts {
		if i > 0 {
			b.WriteString(", ")
		}
		e.writeExpr(b, el)
	}
	if len(lit.Elts) == 0 {
		b.WriteString(e.zeroCValue(elemT))
	}
	fmt.Fprintf(b, "}, %dLL, %dLL }", len(lit.Elts), len(lit.Elts))
}

// writeMapLit emits a GNU statement-expression that builds a map (gcc -std=c11).
func (e *emitter) writeMapLit(b *strings.Builder, lit *ast.CompositeLit, mt check.Type) {
	e.needMap = true
	e.needArena = true
	mn := e.mapCName(mt)
	b.WriteString("({ ")
	b.WriteString(mn)
	b.WriteString(" _ml = ")
	b.WriteString(e.mapMakeName(mt))
	fmt.Fprintf(b, "(%dLL); ", len(lit.Elts))
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		b.WriteString(e.mapSetName(mt))
		b.WriteString("(_ml, ")
		e.writeExpr(b, kv.Key)
		b.WriteString(", ")
		e.writeExpr(b, kv.Value)
		b.WriteString("); ")
	}
	b.WriteString("_ml; })")
}

func (e *emitter) zeroCValue(t check.Type) string {
	switch {
	case t == check.TypeString:
		return "(const char *)0"
	case check.IsMap(t):
		return "NULL"
	case check.IsSlice(t), check.IsArray(t):
		return "{0}"
	case e.info != nil && e.info.Structs[t] != nil:
		return "{0}"
	default:
		return "0"
	}
}

func (e *emitter) writeIndexGet(b *strings.Builder, idx *ast.IndexExpr) {
	xt := e.typeOf(idx.X)
	if check.IsMap(xt) {
		e.needMap = true
		rt := e.typeOf(idx)
		if check.IsTuple(rt) {
			b.WriteString(e.mapGetOkName(xt))
		} else {
			b.WriteString(e.mapGetName(xt))
		}
		b.WriteByte('(')
		e.writeExpr(b, idx.X)
		b.WriteString(", ")
		e.writeExpr(b, idx.Index)
		b.WriteByte(')')
		return
	}
	if check.IsArray(xt) {
		b.WriteString(e.arrayCName(xt))
		b.WriteString("_get(")
		e.writeExpr(b, idx.X)
		b.WriteString(", ")
		e.writeExpr(b, idx.Index)
		b.WriteByte(')')
		return
	}
	e.needSlice = true
	b.WriteString(e.sliceGetName(xt))
	b.WriteByte('(')
	e.writeExpr(b, idx.X)
	b.WriteString(", ")
	e.writeExpr(b, idx.Index)
	b.WriteByte(')')
}

func (e *emitter) writeIndexSet(b *strings.Builder, idx *ast.IndexExpr, val ast.Expr) {
	var vb strings.Builder
	e.writeExpr(&vb, val)
	e.writeIndexSetName(b, idx, vb.String())
}

func (e *emitter) writeIndexSetName(b *strings.Builder, idx *ast.IndexExpr, valC string) {
	xt := e.typeOf(idx.X)
	if check.IsMap(xt) {
		e.needMap = true
		b.WriteString(e.mapSetName(xt))
		b.WriteByte('(')
		e.writeExpr(b, idx.X)
		b.WriteString(", ")
		e.writeExpr(b, idx.Index)
		b.WriteString(", ")
		b.WriteString(valC)
		b.WriteByte(')')
		return
	}
	if check.IsArray(xt) {
		b.WriteString(e.arrayCName(xt))
		b.WriteString("_set(&(")
		e.writeExpr(b, idx.X)
		b.WriteString("), ")
		e.writeExpr(b, idx.Index)
		b.WriteString(", ")
		b.WriteString(valC)
		b.WriteByte(')')
		return
	}
	e.needSlice = true
	b.WriteString(e.sliceSetName(xt))
	b.WriteByte('(')
	e.writeExpr(b, idx.X)
	b.WriteString(", ")
	e.writeExpr(b, idx.Index)
	b.WriteString(", ")
	b.WriteString(valC)
	b.WriteByte(')')
}

func (e *emitter) collectStringConcatParts(expr ast.Expr, parts *[]ast.Expr) {
	if binary, ok := expr.(*ast.BinaryExpr); ok && binary.Op == token.ADD && e.typeOf(binary) == check.TypeString {
		e.collectStringConcatParts(binary.X, parts)
		e.collectStringConcatParts(binary.Y, parts)
		return
	}
	*parts = append(*parts, expr)
}

func (e *emitter) writeBinary(b *strings.Builder, expr *ast.BinaryExpr) {
	lt := e.typeOf(expr.X)
	rt := e.typeOf(expr.Y)
	if expr.Op == token.ADD && (lt == check.TypeString || rt == check.TypeString) {
		e.needConcat = true
		e.needArena = true
		var parts []ast.Expr
		e.collectStringConcatParts(expr, &parts)
		b.WriteString("({ const char *__uli_parts[")
		b.WriteString(fmt.Sprintf("%d", len(parts)))
		b.WriteString("]; ")
		for i, part := range parts {
			b.WriteString("__uli_parts[")
			b.WriteString(fmt.Sprintf("%d", i))
			b.WriteString("] = ")
			e.writeExpr(b, part)
			b.WriteString("; ")
		}
		b.WriteString("uli_concat_many(")
		b.WriteString(fmt.Sprintf("%d", len(parts)))
		b.WriteString(", __uli_parts); })")
		return
	}
	if (expr.Op == token.EQL || expr.Op == token.NEQ) && lt == check.TypeString {
		b.WriteByte('(')
		b.WriteString("strcmp(")
		e.writeExpr(b, expr.X)
		b.WriteString(", ")
		e.writeExpr(b, expr.Y)
		if expr.Op == token.EQL {
			b.WriteString(") == 0")
		} else {
			b.WriteString(") != 0")
		}
		b.WriteByte(')')
		return
	}
	if (expr.Op == token.EQL || expr.Op == token.NEQ) && e.info != nil {
		if si, ok := e.info.Structs[lt]; ok {
			if expr.Op == token.NEQ {
				b.WriteByte('!')
			}
			b.WriteString(e.eqFuncName(si))
			b.WriteByte('(')
			e.writeExpr(b, expr.X)
			b.WriteString(", ")
			e.writeExpr(b, expr.Y)
			b.WriteByte(')')
			return
		}
	}
	// Mixed integer / மிதவைஎண்: promote integer side to double.
	isIntish := func(t check.Type) bool {
		return t == check.TypeInt || t == check.TypeByte || t == check.TypeRune ||
			t == check.TypeInt8 || t == check.TypeInt16 || t == check.TypeInt32 || t == check.TypeInt64 ||
			t == check.TypeUint8 || t == check.TypeUint16 || t == check.TypeUint32 || t == check.TypeUint64
	}
	mixFloat := (isIntish(lt) || lt == check.TypeFloat) &&
		(isIntish(rt) || rt == check.TypeFloat) &&
		(lt == check.TypeFloat || rt == check.TypeFloat)
	b.WriteByte('(')
	if mixFloat && isIntish(lt) {
		b.WriteString("(double)(")
		e.writeExpr(b, expr.X)
		b.WriteByte(')')
	} else {
		e.writeExpr(b, expr.X)
	}
	if expr.Op == token.AND_NOT {
		b.WriteString(" & ~(")
		if mixFloat && isIntish(rt) {
			b.WriteString("(double)(")
			e.writeExpr(b, expr.Y)
			b.WriteString(")")
		} else {
			e.writeExpr(b, expr.Y)
		}
		b.WriteString("))")
		return
	}
	b.WriteByte(' ')
	b.WriteString(opString(expr.Op))
	b.WriteByte(' ')
	if mixFloat && isIntish(rt) {
		b.WriteString("(double)(")
		e.writeExpr(b, expr.Y)
		b.WriteByte(')')
	} else {
		e.writeExpr(b, expr.Y)
	}
	b.WriteByte(')')
}

func (e *emitter) writePrint(b *strings.Builder, arg ast.Expr) {
	t := e.typeOf(arg)
	if e.info != nil {
		if si, ok := e.info.Structs[t]; ok {
			b.WriteString(e.printFuncName(si))
			b.WriteByte('(')
			e.writeExpr(b, arg)
			b.WriteString("); printf(\"\\n\")")
			return
		}
		if under, ok := e.info.Underlying[t]; ok {
			t = under
		}
	}
	switch t {
	case check.TypeString:
		b.WriteString("uli_print_str(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case check.TypeBool:
		b.WriteString("uli_print_bool(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case check.TypeFloat:
		b.WriteString("uli_print_float(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case check.TypeByte, check.TypeRune,
		check.TypeInt8, check.TypeInt16, check.TypeInt32, check.TypeInt64,
		check.TypeUint8, check.TypeUint16, check.TypeUint32, check.TypeUint64:
		b.WriteString("uli_print_int((int64_t)(")
		e.writeExpr(b, arg)
		b.WriteString("))")
	default:
		switch arg := arg.(type) {
		case *ast.BasicLit:
			if arg.Kind == token.STRING {
				b.WriteString("uli_print_str(")
				e.writeExpr(b, arg)
				b.WriteByte(')')
				return
			}
			if arg.Kind == token.FLOAT {
				b.WriteString("uli_print_float(")
				e.writeExpr(b, arg)
				b.WriteByte(')')
				return
			}
		case *ast.BoolLit:
			b.WriteString("uli_print_bool(")
			e.writeExpr(b, arg)
			b.WriteByte(')')
			return
		}
		if isBoolish(arg) {
			b.WriteString("uli_print_bool(")
			e.writeExpr(b, arg)
			b.WriteByte(')')
			return
		}
		b.WriteString("uli_print_int(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	}
}

func isBoolish(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BoolLit:
		return true
	case *ast.UnaryExpr:
		return e.Op == token.NOT
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return true
		}
	}
	return false
}

func opString(op token.Kind) string {
	switch op {
	case token.ADD:
		return "+"
	case token.SUB:
		return "-"
	case token.MUL:
		return "*"
	case token.QUO:
		return "/"
	case token.REM:
		return "%"
	case token.AND:
		return "&"
	case token.OR:
		return "|"
	case token.XOR:
		return "^"
	case token.AND_NOT:
		return "&~"
	case token.SHL:
		return "<<"
	case token.SHR:
		return ">>"
	case token.EQL:
		return "=="
	case token.NEQ:
		return "!="
	case token.LSS:
		return "<"
	case token.LEQ:
		return "<="
	case token.GTR:
		return ">"
	case token.GEQ:
		return ">="
	default:
		return "?"
	}
}

func escapeCString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
