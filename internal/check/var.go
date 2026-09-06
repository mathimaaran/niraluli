package check

import (
	"niraluli/internal/ast"
	"niraluli/internal/token"
)

// pkgVar is one package-level மாறி.
type pkgVar struct {
	typ      Type
	exported bool
}

// PkgVarValueInfo records a package variable reference for emit.
type PkgVarValueInfo struct {
	Pkg  string
	Name string
}

func (c *Checker) collectPackageVars(f *ast.File) {
	if c.cur == nil {
		return
	}
	if c.cur.vars == nil {
		c.cur.vars = map[string]*pkgVar{}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.VarDecl:
			c.definePackageVar(d)
		case *ast.VarGroupDecl:
			c.definePackageVarGroup(d)
		}
	}
}

func (c *Checker) definePackageVar(d *ast.VarDecl) {
	if d == nil || c.cur == nil {
		return
	}
	c.definePackageVarSpec(&ast.VarSpec{
		Names:  d.Names,
		Type:   d.Type,
		Values: d.Values,
	}, d.Exported, d.Pos())
}

func (c *Checker) definePackageVarGroup(d *ast.VarGroupDecl) {
	if d == nil || c.cur == nil {
		return
	}
	for _, spec := range d.Specs {
		c.definePackageVarSpec(spec, d.Exported, d.Pos())
	}
}

func (c *Checker) definePackageVarSpec(spec *ast.VarSpec, exported bool, pos token.Pos) {
	if spec == nil {
		return
	}
	var t Type = TypeInvalid
	if spec.Type != nil {
		t = c.typeFromExpr(spec.Type)
		if t == TypeInvalid {
			c.error(spec.Type.Pos(), "invalid type")
			return
		}
	}
	if len(spec.Values) != 0 && len(spec.Values) != len(spec.Names) {
		c.error(pos, "wrong number of initializers")
		return
	}
	if spec.Type == nil && len(spec.Values) == 0 {
		c.error(pos, "variable declaration requires a type or initializer")
		return
	}
	for i, name := range spec.Names {
		if name == nil {
			continue
		}
		if _, exists := c.cur.vars[name.Name]; exists {
			c.error(name.Pos(), "variable redeclared: %s", name.Name)
			continue
		}
		if _, exists := c.cur.consts[name.Name]; exists {
			c.error(name.Pos(), "already declared: %s", name.Name)
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
		nt := t
		if i < len(spec.Values) {
			vt := c.checkExpr(spec.Values[i])
			if spec.Type == nil {
				if vt == TypeVoid || vt == TypeInvalid {
					c.error(spec.Values[i].Pos(), "cannot infer type for %s", name.Name)
					continue
				}
				nt = vt
			} else if !c.assignable(vt, t, spec.Values[i]) {
				c.error(spec.Values[i].Pos(), "cannot initialize %s as %s", c.typStr(t), c.typStr(vt))
				continue
			}
		}
		c.cur.vars[name.Name] = &pkgVar{typ: nt, exported: exported}
		if c.info != nil {
			if c.info.PkgVarTypes == nil {
				c.info.PkgVarTypes = map[string]Type{}
			}
			c.info.PkgVarTypes[c.cur.name+"."+name.Name] = nt
		}
	}
}

func (c *Checker) lookupPkgVar(name string) (*pkgVar, bool) {
	if c.cur == nil {
		return nil, false
	}
	pv, ok := c.cur.vars[name]
	return pv, ok
}

func (c *Checker) recordPkgVarRef(e ast.Expr, pkg, name string) {
	if e == nil || c.info == nil {
		return
	}
	if c.info.PkgVarValues == nil {
		c.info.PkgVarValues = map[ast.Expr]*PkgVarValueInfo{}
	}
	c.info.PkgVarValues[e] = &PkgVarValueInfo{Pkg: pkg, Name: name}
}

func (c *Checker) isPkgVarIdent(name string) bool {
	_, ok := c.lookupPkgVar(name)
	return ok
}
