package check

import (
	"niraluli/internal/ast"
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
		vd, ok := d.(*ast.VarDecl)
		if !ok {
			continue
		}
		c.definePackageVar(vd)
	}
}

func (c *Checker) definePackageVar(d *ast.VarDecl) {
	if d == nil || c.cur == nil {
		return
	}
	t := c.typeFromExpr(d.Type)
	if t == TypeInvalid {
		c.error(d.Type.Pos(), "invalid type")
		return
	}
	if len(d.Values) != 0 && len(d.Values) != len(d.Names) {
		c.error(d.Pos(), "wrong number of initializers")
		return
	}
	for i, name := range d.Names {
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
		if i < len(d.Values) {
			vt := c.checkExpr(d.Values[i])
			if !c.assignable(vt, t, d.Values[i]) {
				c.error(d.Values[i].Pos(), "cannot initialize %s as %s", c.typStr(t), c.typStr(vt))
				continue
			}
		}
		c.cur.vars[name.Name] = &pkgVar{typ: t, exported: d.Exported}
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
