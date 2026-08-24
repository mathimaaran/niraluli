package check

import (
	"strconv"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/token"
)

// ConstKind classifies a folded constant value.
type ConstKind int

const (
	ConstInvalid ConstKind = iota
	ConstInt
	ConstFloat
	ConstBool
	ConstString
)

// ConstValue is a compile-time constant (Tamil-0.67).
type ConstValue struct {
	Kind  ConstKind
	Int   int64
	Float float64
	Bool  bool
	Str   string
}

// pkgConst is one package-level மாறிலி.
type pkgConst struct {
	typ      Type
	val      ConstValue
	exported bool
}

func (s *scope) declareConst(name string, t Type, v ConstValue, pos token.Pos, errs *[]error) {
	if _, exists := s.vars[name]; exists {
		*errs = append(*errs, Error{Pos: pos, Msg: "already declared: " + name})
		return
	}
	if s.constVals == nil {
		s.constVals = map[string]ConstValue{}
	}
	s.vars[name] = t
	s.constVals[name] = v
}

func (s *scope) lookupConst(name string) (ConstValue, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if v, ok := cur.constVals[name]; ok {
			return v, true
		}
	}
	return ConstValue{}, false
}

func (s *scope) isConstName(name string) bool {
	_, ok := s.lookupConst(name)
	return ok
}

func (c *Checker) lookupPkgConst(name string) (*pkgConst, bool) {
	if c.cur == nil {
		return nil, false
	}
	pc, ok := c.cur.consts[name]
	return pc, ok
}

func (c *Checker) isConstIdent(name string) bool {
	if c.scope != nil && c.scope.isConstName(name) {
		return true
	}
	_, ok := c.lookupPkgConst(name)
	return ok
}

func (c *Checker) constOfIdent(name string) (ConstValue, Type, bool) {
	if c.scope != nil {
		if v, ok := c.scope.lookupConst(name); ok {
			t, _ := c.scope.lookup(name)
			return v, t, true
		}
	}
	if pc, ok := c.lookupPkgConst(name); ok {
		return pc.val, pc.typ, true
	}
	return ConstValue{}, TypeInvalid, false
}

func (c *Checker) recordConstExpr(e ast.Expr, v ConstValue) {
	if e == nil || v.Kind == ConstInvalid || c.info == nil {
		return
	}
	if c.info.ConstExprs == nil {
		c.info.ConstExprs = map[ast.Expr]ConstValue{}
	}
	c.info.ConstExprs[e] = v
}

func (c *Checker) collectPackageConsts(f *ast.File) {
	if c.cur == nil {
		return
	}
	if c.cur.consts == nil {
		c.cur.consts = map[string]*pkgConst{}
	}
	for _, d := range f.Decls {
		cd, ok := d.(*ast.ConstDecl)
		if !ok {
			continue
		}
		c.defineConstDecl(cd, true)
	}
}

func (c *Checker) checkConstDecl(d *ast.ConstDecl) {
	c.defineConstDecl(d, false)
}

func (c *Checker) defineConstDecl(d *ast.ConstDecl, pkgLevel bool) {
	if d == nil {
		return
	}
	if len(d.Values) != len(d.Names) {
		c.error(d.Pos(), "wrong number of constant initializers")
		return
	}
	var want Type
	if d.Type != nil {
		want = c.typeFromExpr(d.Type)
		if want == TypeInvalid {
			c.error(d.Type.Pos(), "invalid constant type")
			return
		}
		if !c.isConstAllowedType(want) {
			c.error(d.Type.Pos(), "constant type must be a scalar (முழுஎண், மிதவைஎண், நிலை, சரம், இருமி8, இருமி32)")
			return
		}
	}
	for i, name := range d.Names {
		if name == nil {
			continue
		}
		v, vt, ok := c.evalConst(d.Values[i])
		if !ok {
			c.error(d.Values[i].Pos(), "constant initializer must be a constant expression")
			continue
		}
		t := want
		if t == TypeInvalid {
			t = vt
		} else if !c.constAssignable(v, vt, t) {
			c.error(d.Values[i].Pos(), "cannot initialize constant %s as %s", c.typStr(t), c.typStr(vt))
			continue
		} else {
			v = c.coerceConst(v, t)
		}
		c.recordConstExpr(d.Values[i], v)
		if pkgLevel {
			if _, exists := c.cur.consts[name.Name]; exists {
				c.error(name.Pos(), "constant redeclared: %s", name.Name)
				continue
			}
			if _, exists := c.cur.funcs[name.Name]; exists {
				c.error(name.Pos(), "already declared: %s", name.Name)
				continue
			}
			if _, exists := c.cur.types[name.Name]; exists {
				c.error(name.Pos(), "already declared: %s", name.Name)
				continue
			}
			c.cur.consts[name.Name] = &pkgConst{typ: t, val: v, exported: d.Exported}
		} else {
			c.scope.declareConst(name.Name, t, v, name.Pos(), &c.errs)
		}
	}
}

func (c *Checker) isConstAllowedType(t Type) bool {
	u := c.underlying(t)
	switch u {
	case TypeInt, TypeFloat, TypeBool, TypeString, TypeByte, TypeRune:
		return true
	default:
		return false
	}
}

func (c *Checker) constAssignable(v ConstValue, got, want Type) bool {
	if got == want {
		return true
	}
	ug, uw := c.underlying(got), c.underlying(want)
	if ug == uw {
		return true
	}
	if v.Kind == ConstInt && (uw == TypeInt || uw == TypeByte || uw == TypeRune || uw == TypeFloat) {
		return true
	}
	if v.Kind == ConstFloat && uw == TypeFloat {
		return true
	}
	if v.Kind == ConstBool && uw == TypeBool {
		return true
	}
	if v.Kind == ConstString && uw == TypeString {
		return true
	}
	return false
}

func (c *Checker) coerceConst(v ConstValue, want Type) ConstValue {
	u := c.underlying(want)
	switch u {
	case TypeFloat:
		if v.Kind == ConstInt {
			return ConstValue{Kind: ConstFloat, Float: float64(v.Int)}
		}
	case TypeByte, TypeRune, TypeInt:
		if v.Kind == ConstInt {
			return v
		}
	}
	return v
}

func (c *Checker) evalConst(e ast.Expr) (ConstValue, Type, bool) {
	if e == nil {
		return ConstValue{}, TypeInvalid, false
	}
	switch e := e.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			n, err := parseIntLit(e.Value)
			if err != nil {
				return ConstValue{}, TypeInvalid, false
			}
			v := ConstValue{Kind: ConstInt, Int: n}
			c.recordConstExpr(e, v)
			return v, TypeInt, true
		case token.FLOAT:
			f, err := strconv.ParseFloat(e.Value, 64)
			if err != nil {
				return ConstValue{}, TypeInvalid, false
			}
			v := ConstValue{Kind: ConstFloat, Float: f}
			c.recordConstExpr(e, v)
			return v, TypeFloat, true
		case token.STRING:
			v := ConstValue{Kind: ConstString, Str: e.Value}
			c.recordConstExpr(e, v)
			return v, TypeString, true
		}
	case *ast.BoolLit:
		v := ConstValue{Kind: ConstBool, Bool: e.Value}
		c.recordConstExpr(e, v)
		return v, TypeBool, true
	case *ast.ParenExpr:
		v, t, ok := c.evalConst(e.X)
		if ok {
			c.recordConstExpr(e, v)
		}
		return v, t, ok
	case *ast.Ident:
		if v, t, ok := c.constOfIdent(e.Name); ok {
			c.recordConstExpr(e, v)
			return v, t, true
		}
		return ConstValue{}, TypeInvalid, false
	case *ast.SelectorExpr:
		id, ok := e.X.(*ast.Ident)
		if !ok || c.cur == nil {
			return ConstValue{}, TypeInvalid, false
		}
		imp, ok := c.cur.imports[id.Name]
		if !ok {
			return ConstValue{}, TypeInvalid, false
		}
		pc, ok := imp.consts[e.Sel.Name]
		if !ok || !pc.exported {
			return ConstValue{}, TypeInvalid, false
		}
		c.markImportUsed(id.Name)
		c.recordConstExpr(e, pc.val)
		return pc.val, pc.typ, true
	case *ast.UnaryExpr:
		xv, xt, ok := c.evalConst(e.X)
		if !ok {
			return ConstValue{}, TypeInvalid, false
		}
		switch e.Op {
		case token.ADD:
			if xv.Kind == ConstInt || xv.Kind == ConstFloat {
				c.recordConstExpr(e, xv)
				return xv, xt, true
			}
		case token.SUB:
			switch xv.Kind {
			case ConstInt:
				v := ConstValue{Kind: ConstInt, Int: -xv.Int}
				c.recordConstExpr(e, v)
				return v, TypeInt, true
			case ConstFloat:
				v := ConstValue{Kind: ConstFloat, Float: -xv.Float}
				c.recordConstExpr(e, v)
				return v, TypeFloat, true
			}
		case token.NOT:
			if xv.Kind == ConstBool {
				v := ConstValue{Kind: ConstBool, Bool: !xv.Bool}
				c.recordConstExpr(e, v)
				return v, TypeBool, true
			}
		}
		return ConstValue{}, TypeInvalid, false
	case *ast.BinaryExpr:
		lv, lt, okL := c.evalConst(e.X)
		rv, rt, okR := c.evalConst(e.Y)
		if !okL || !okR {
			return ConstValue{}, TypeInvalid, false
		}
		switch e.Op {
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			b, ok := cmpConst(lv, rv, e.Op)
			if !ok {
				return ConstValue{}, TypeInvalid, false
			}
			v := ConstValue{Kind: ConstBool, Bool: b}
			c.recordConstExpr(e, v)
			return v, TypeBool, true
		case token.ADD:
			if lv.Kind == ConstString && rv.Kind == ConstString {
				v := ConstValue{Kind: ConstString, Str: lv.Str + rv.Str}
				c.recordConstExpr(e, v)
				return v, TypeString, true
			}
			fallthrough
		case token.SUB, token.MUL, token.QUO, token.REM:
			v, t, ok := arithConst(lv, lt, rv, rt, e.Op)
			if !ok {
				return ConstValue{}, TypeInvalid, false
			}
			c.recordConstExpr(e, v)
			return v, t, true
		}
	}
	return ConstValue{}, TypeInvalid, false
}

func parseIntLit(lit string) (int64, error) {
	s := strings.Map(func(r rune) rune {
		if r >= '௦' && r <= '௯' {
			return '0' + (r - '௦')
		}
		return r
	}, lit)
	return strconv.ParseInt(s, 0, 64)
}

func arithConst(lv ConstValue, lt Type, rv ConstValue, rt Type, op token.Kind) (ConstValue, Type, bool) {
	if lv.Kind == ConstFloat || rv.Kind == ConstFloat || lt == TypeFloat || rt == TypeFloat {
		lf, rf := constAsFloat(lv), constAsFloat(rv)
		var f float64
		switch op {
		case token.ADD:
			f = lf + rf
		case token.SUB:
			f = lf - rf
		case token.MUL:
			f = lf * rf
		case token.QUO:
			if rf == 0 {
				return ConstValue{}, TypeInvalid, false
			}
			f = lf / rf
		default:
			return ConstValue{}, TypeInvalid, false
		}
		return ConstValue{Kind: ConstFloat, Float: f}, TypeFloat, true
	}
	if lv.Kind != ConstInt || rv.Kind != ConstInt {
		return ConstValue{}, TypeInvalid, false
	}
	a, b := lv.Int, rv.Int
	var n int64
	switch op {
	case token.ADD:
		n = a + b
	case token.SUB:
		n = a - b
	case token.MUL:
		n = a * b
	case token.QUO:
		if b == 0 {
			return ConstValue{}, TypeInvalid, false
		}
		n = a / b
	case token.REM:
		if b == 0 {
			return ConstValue{}, TypeInvalid, false
		}
		n = a % b
	default:
		return ConstValue{}, TypeInvalid, false
	}
	return ConstValue{Kind: ConstInt, Int: n}, TypeInt, true
}

func constAsFloat(v ConstValue) float64 {
	if v.Kind == ConstFloat {
		return v.Float
	}
	return float64(v.Int)
}

func cmpConst(lv, rv ConstValue, op token.Kind) (bool, bool) {
	if lv.Kind == ConstString && rv.Kind == ConstString {
		switch op {
		case token.EQL:
			return lv.Str == rv.Str, true
		case token.NEQ:
			return lv.Str != rv.Str, true
		default:
			return false, false
		}
	}
	if lv.Kind == ConstBool && rv.Kind == ConstBool {
		switch op {
		case token.EQL:
			return lv.Bool == rv.Bool, true
		case token.NEQ:
			return lv.Bool != rv.Bool, true
		default:
			return false, false
		}
	}
	if (lv.Kind == ConstInt || lv.Kind == ConstFloat) && (rv.Kind == ConstInt || rv.Kind == ConstFloat) {
		lf, rf := constAsFloat(lv), constAsFloat(rv)
		switch op {
		case token.EQL:
			return lf == rf, true
		case token.NEQ:
			return lf != rf, true
		case token.LSS:
			return lf < rf, true
		case token.GTR:
			return lf > rf, true
		case token.LEQ:
			return lf <= rf, true
		case token.GEQ:
			return lf >= rf, true
		}
	}
	return false, false
}
