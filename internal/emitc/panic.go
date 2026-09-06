package emitc

import (
	"fmt"
	"strconv"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/check"
)

func (e *emitter) fnHasUnwind() bool {
	return e.fnHasDefer || e.needPanic
}

func (e *emitter) writePanicCall(b *strings.Builder, arg ast.Expr) {
	t := e.peelUnderlying(e.typeOf(arg))
	switch t {
	case check.TypeString:
		b.WriteString("uli_panic(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case check.TypeBool:
		b.WriteString("uli_panic_bool(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	case check.TypeFloat:
		b.WriteString("uli_panic_float(")
		e.writeExpr(b, arg)
		b.WriteByte(')')
	default:
		b.WriteString("uli_panic_int((int64_t)(")
		e.writeExpr(b, arg)
		b.WriteString("))")
	}
}

func (e *emitter) writePanicCallRef(b *strings.Builder, t check.Type, ref string) {
	t = e.peelUnderlying(t)
	switch t {
	case check.TypeString:
		fmt.Fprintf(b, "uli_panic(%s)", ref)
	case check.TypeBool:
		fmt.Fprintf(b, "uli_panic_bool(%s)", ref)
	case check.TypeFloat:
		fmt.Fprintf(b, "uli_panic_float(%s)", ref)
	default:
		fmt.Fprintf(b, "uli_panic_int((int64_t)(%s))", ref)
	}
}

func (e *emitter) markPanicNeeds(pkgs []*check.PkgInfo) {
	for _, p := range pkgs {
		if p == nil || p.File == nil {
			continue
		}
		for _, d := range p.File.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if hasPanicInStmt(d.Body) {
					e.needPanic = true
					return
				}
			case *ast.VarDecl:
				for _, v := range d.Values {
					if hasPanicInExpr(v) {
						e.needPanic = true
						return
					}
				}
			case *ast.VarGroupDecl:
				for _, spec := range d.Specs {
					for _, v := range spec.Values {
						if hasPanicInExpr(v) {
							e.needPanic = true
							return
						}
					}
				}
			}
		}
	}
}

func hasPanicInStmt(s ast.Stmt) bool {
	if s == nil {
		return false
	}
	switch s := s.(type) {
	case *ast.BlockStmt:
		for _, st := range s.List {
			if hasPanicInStmt(st) {
				return true
			}
		}
	case *ast.IfStmt:
		return hasPanicInStmt(s.Init) || hasPanicInExpr(s.Cond) || hasPanicInStmt(s.Body) || hasPanicInStmt(s.Else)
	case *ast.SwitchStmt:
		if hasPanicInStmt(s.Init) || hasPanicInExpr(s.Tag) {
			return true
		}
		for _, c := range s.Cases {
			if c == nil {
				continue
			}
			for _, x := range c.List {
				if hasPanicInExpr(x) {
					return true
				}
			}
			if hasPanicInStmt(c.Body) {
				return true
			}
		}
	case *ast.ForStmt:
		return hasPanicInStmt(s.Init) || hasPanicInExpr(s.Cond) || hasPanicInStmt(s.Post) || hasPanicInStmt(s.Body)
	case *ast.RangeStmt:
		return hasPanicInExpr(s.X) || hasPanicInStmt(s.Body)
	case *ast.ReturnStmt:
		for _, v := range s.Results {
			if hasPanicInExpr(v) {
				return true
			}
		}
	case *ast.DeferStmt:
		if s.Call != nil && hasPanicInExpr(s.Call) {
			return true
		}
	case *ast.GoStmt:
		if s.Call != nil && hasPanicInExpr(s.Call) {
			return true
		}
	case *ast.SendStmt:
		return hasPanicInExpr(s.Chan) || hasPanicInExpr(s.Value)
	case *ast.SelectStmt:
		for _, c := range s.Body {
			if c == nil {
				continue
			}
			if hasPanicInStmt(c.Comm) || hasPanicInStmt(c.Body) {
				return true
			}
		}
	case *ast.AssignStmt:
		for _, x := range s.LHS {
			if hasPanicInExpr(x) {
				return true
			}
		}
		for _, x := range s.Values {
			if hasPanicInExpr(x) {
				return true
			}
		}
	case *ast.ShortVarDecl:
		for _, x := range s.Values {
			if hasPanicInExpr(x) {
				return true
			}
		}
	case *ast.ExprStmt:
		return hasPanicInExpr(s.X)
	case *ast.VarDecl:
		for _, x := range s.Values {
			if hasPanicInExpr(x) {
				return true
			}
		}
	case *ast.VarGroupDecl:
		for _, spec := range s.Specs {
			for _, x := range spec.Values {
				if hasPanicInExpr(x) {
					return true
				}
			}
		}
	}
	return false
}

func hasPanicInExpr(x ast.Expr) bool {
	if x == nil {
		return false
	}
	switch x := x.(type) {
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok {
			if id.Name == "அலறு" || id.Name == "மீள்" {
				return true
			}
		}
		if hasPanicInExpr(x.Fun) {
			return true
		}
		for _, a := range x.Args {
			if hasPanicInExpr(a) {
				return true
			}
		}
	case *ast.FuncLit:
		return hasPanicInStmt(x.Body)
	case *ast.UnaryExpr:
		return hasPanicInExpr(x.X)
	case *ast.BinaryExpr:
		return hasPanicInExpr(x.X) || hasPanicInExpr(x.Y)
	case *ast.ParenExpr:
		return hasPanicInExpr(x.X)
	case *ast.IndexExpr:
		return hasPanicInExpr(x.X) || hasPanicInExpr(x.Index)
	case *ast.SliceExpr:
		return hasPanicInExpr(x.X) || hasPanicInExpr(x.Low) || hasPanicInExpr(x.High) || hasPanicInExpr(x.Max)
	case *ast.SelectorExpr:
		return hasPanicInExpr(x.X)
	case *ast.CompositeLit:
		for _, el := range x.Elts {
			if hasPanicInExpr(el) {
				return true
			}
		}
	case *ast.KeyValueExpr:
		return hasPanicInExpr(x.Key) || hasPanicInExpr(x.Value)
	}
	return false
}

func (e *emitter) panicFrameName(fn *ast.FuncDecl) string {
	if fn == nil || fn.Name == nil {
		return "செயல்பாடு"
	}
	if fn.Recv != nil && fn.Recv.Type != nil {
		switch t := fn.Recv.Type.(type) {
		case *ast.TypeName:
			return t.Name + "." + fn.Name.Name
		case *ast.PointerType:
			if tn, ok := t.Elem.(*ast.TypeName); ok {
				return "*" + tn.Name + "." + fn.Name.Name
			}
		}
	}
	return fn.Name.Name
}

func (e *emitter) writeUnwindPrologue(b *strings.Builder, fn *ast.FuncDecl, frame string) {
	if !e.fnHasUnwind() {
		return
	}
	if e.fnHasDefer {
		e.needDefer = true
		e.needArena = true
		b.WriteString("\tuli_defer_frame *volatile _defers = NULL;\n")
	}
	rts := e.resultTypes(fn)
	if len(rts) > 0 && !e.fnNamedResults() {
		b.WriteByte('\t')
		switch len(rts) {
		case 1:
			b.WriteString(e.cTypeFrom(rts[0]))
		default:
			b.WriteString(e.retCName(rts))
		}
		b.WriteString(" _ret;\n")
		b.WriteString("\tmemset(&_ret, 0, sizeof(_ret));\n")
	}
	if e.needPanic {
		if frame == "" {
			frame = e.panicFrameName(fn)
		}
		b.WriteString("\tint _save_defer = uli_is_defer_fn;\n")
		b.WriteString("\tif (uli_defer_pending) {\n")
		b.WriteString("\t\tuli_defer_pending = 0;\n")
		b.WriteString("\t\tuli_is_defer_fn = 1;\n")
		b.WriteString("\t} else {\n")
		b.WriteString("\t\tuli_is_defer_fn = 0;\n")
		b.WriteString("\t}\n")
		fmt.Fprintf(b, "\tuli_panic_push(%s);\n", strconv.Quote(frame))
		b.WriteString("\tif (setjmp(*uli_jmp_push()) != 0) goto __uli_epilogue;\n")
	}
}

func (e *emitter) writePanicRuntime(b *strings.Builder) {
	if !e.needPanic {
		return
	}
	b.WriteString("#define ULI_JMP_MAX 64\n")
	b.WriteString("static _Thread_local jmp_buf uli_jmps[ULI_JMP_MAX];\n")
	b.WriteString("static _Thread_local int uli_jmp_n;\n")
	b.WriteString("static _Thread_local const char *uli_panic_msg;\n")
	b.WriteString("static _Thread_local int uli_panic_on;\n")
	b.WriteString("static _Thread_local int uli_is_defer_fn;\n")
	b.WriteString("static _Thread_local int uli_defer_pending;\n")
	b.WriteString("static _Thread_local const char *uli_pstack[ULI_JMP_MAX];\n")
	b.WriteString("static _Thread_local int uli_pstack_n;\n")
	b.WriteString("static _Thread_local const char *uli_panic_snap[ULI_JMP_MAX];\n")
	b.WriteString("static _Thread_local int uli_panic_snap_n;\n")
	b.WriteString("static _Thread_local char uli_panic_buf[128];\n")
	b.WriteString("static void uli_panic_push(const char *name) {\n")
	b.WriteString("\tif (uli_pstack_n < ULI_JMP_MAX) uli_pstack[uli_pstack_n++] = name ? name : \"?\";\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic_pop(void) {\n")
	b.WriteString("\tif (uli_pstack_n > 0) uli_pstack_n--;\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic_dump(void) {\n")
	b.WriteString("\tfprintf(stderr, \"அலறு: %s\\n\", uli_panic_msg ? uli_panic_msg : \"\");\n")
	b.WriteString("\tfor (int i = uli_panic_snap_n - 1; i >= 0; i--) {\n")
	b.WriteString("\t\tfprintf(stderr, \"  %s\\n\", uli_panic_snap[i] ? uli_panic_snap[i] : \"?\");\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	b.WriteString("static jmp_buf *uli_jmp_push(void) {\n")
	b.WriteString("\tif (uli_jmp_n >= ULI_JMP_MAX) {\n")
	b.WriteString("\t\tfprintf(stderr, \"அலறு: jmp stack overflow\\n\");\n")
	b.WriteString("\t\tabort();\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn &uli_jmps[uli_jmp_n++];\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_jmp_pop(void) {\n")
	b.WriteString("\tif (uli_jmp_n > 0) uli_jmp_n--;\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic(const char *msg) {\n")
	b.WriteString("\tuli_panic_msg = msg ? msg : \"\";\n")
	b.WriteString("\tuli_panic_on = 1;\n")
	b.WriteString("\tuli_panic_snap_n = uli_pstack_n;\n")
	b.WriteString("\tfor (int i = 0; i < uli_pstack_n && i < ULI_JMP_MAX; i++) uli_panic_snap[i] = uli_pstack[i];\n")
	b.WriteString("\tif (uli_jmp_n <= 0) {\n")
	b.WriteString("\t\tuli_panic_dump();\n")
	b.WriteString("\t\tabort();\n")
	b.WriteString("\t}\n")
	b.WriteString("\tlongjmp(uli_jmps[uli_jmp_n - 1], 1);\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic_int(int64_t v) {\n")
	b.WriteString("\tsnprintf(uli_panic_buf, sizeof uli_panic_buf, \"%lld\", (long long)v);\n")
	b.WriteString("\tuli_panic(uli_panic_buf);\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic_float(double v) {\n")
	b.WriteString("\tsnprintf(uli_panic_buf, sizeof uli_panic_buf, \"%g\", v);\n")
	b.WriteString("\tuli_panic(uli_panic_buf);\n")
	b.WriteString("}\n")
	b.WriteString("static void uli_panic_bool(int v) {\n")
	b.WriteString("\tuli_panic(v ? \"மெய்\" : \"பொய்\");\n")
	b.WriteString("}\n")
	b.WriteString("static const char *uli_recover(void) {\n")
	b.WriteString("\tif (!uli_is_defer_fn || !uli_panic_on) return \"\";\n")
	b.WriteString("\tuli_panic_on = 0;\n")
	b.WriteString("\t{\n")
	b.WriteString("\t\tconst char *m = uli_panic_msg ? uli_panic_msg : \"\";\n")
	b.WriteString("\t\tuli_panic_msg = NULL;\n")
	b.WriteString("\t\treturn m;\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
}

func (e *emitter) writePanicRethrow(b *strings.Builder) {
	if !e.needPanic {
		return
	}
	b.WriteString("\tuli_is_defer_fn = _save_defer;\n")
	b.WriteString("\tif (uli_panic_on) {\n")
	b.WriteString("\t\tuli_jmp_pop();\n")
	b.WriteString("\t\tif (uli_jmp_n <= 0) {\n")
	b.WriteString("\t\t\tuli_panic_dump();\n")
	b.WriteString("\t\t\tabort();\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tuli_panic_pop();\n")
	b.WriteString("\t\tlongjmp(uli_jmps[uli_jmp_n - 1], 1);\n")
	b.WriteString("\t}\n")
	b.WriteString("\tuli_panic_pop();\n")
	b.WriteString("\tuli_jmp_pop();\n")
}
