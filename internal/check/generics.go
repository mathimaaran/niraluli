package check

import (
	"niraluli/internal/ast"
	"niraluli/internal/token"
)

// typeConstraint limits a type parameter (Tamil-0.71).
// Empty union means unconstrained (any).
type typeConstraint struct {
	union []Type
}

func (c *Checker) resolveConstraint(tp *ast.TypeParam) typeConstraint {
	if tp == nil || len(tp.ConstraintTypes) == 0 {
		return typeConstraint{}
	}
	out := typeConstraint{}
	for _, te := range tp.ConstraintTypes {
		t := c.typeFromExpr(te)
		if t == TypeInvalid || t == TypeVoid {
			c.error(te.Pos(), "invalid constraint type")
			continue
		}
		if IsTypeParam(t) {
			c.error(te.Pos(), "constraint must name concrete types")
			continue
		}
		out.union = append(out.union, t)
	}
	return out
}

func (c *Checker) satisfiesConstraint(t Type, con typeConstraint) bool {
	if len(con.union) == 0 {
		return t != TypeInvalid && t != TypeVoid
	}
	for _, u := range con.union {
		if c.sameType(t, u) {
			return true
		}
	}
	return false
}

func (c *Checker) checkTypeArgsConstraints(typeArgs []Type, constraints []typeConstraint, pos token.Pos, what string) {
	for i := range constraints {
		if i >= len(typeArgs) {
			break
		}
		if typeArgs[i] == TypeInvalid {
			continue
		}
		if !c.satisfiesConstraint(typeArgs[i], constraints[i]) {
			c.error(pos, "type argument %d does not satisfy constraint for %s (got %s)",
				i+1, what, c.typStr(typeArgs[i]))
		}
	}
}

type genericTypeSchema struct {
	decl          *ast.TypeDecl
	typeParams    []Type
	paramNames    []string
	constraints   []typeConstraint
	isStruct      bool
	isAlias       bool
	schematicType Type
}

func (c *Checker) typeArgsAreSchemaParams(exprs []ast.TypeExpr, sch *genericTypeSchema) bool {
	if sch == nil || len(exprs) != len(sch.typeParams) {
		return false
	}
	for i, te := range exprs {
		tn, ok := te.(*ast.TypeName)
		if !ok || tn.Pkg != nil || len(tn.TypeArgs) > 0 {
			return false
		}
		if tn.Name != sch.paramNames[i] {
			return false
		}
	}
	return true
}

func (c *Checker) registerGenericType(td *ast.TypeDecl) {
	name := td.Name.Name
	if _, exists := c.cur.genericTypes[name]; exists {
		c.error(td.Name.Pos(), "type redeclared: %s", name)
		return
	}
	schema := &genericTypeSchema{
		decl:    td,
		isAlias: td.Alias,
	}
	prevEnv := c.typeParamEnv
	c.typeParamEnv = map[string]Type{}
	seen := map[string]bool{}
	for _, tp := range td.TypeParams {
		if tp == nil || tp.Name == nil {
			continue
		}
		pname := tp.Name.Name
		if seen[pname] {
			c.error(tp.Name.Pos(), "duplicate type parameter %s", pname)
			continue
		}
		seen[pname] = true
		t := c.allocTypeParam(pname)
		c.typeParamEnv[pname] = t
		schema.typeParams = append(schema.typeParams, t)
		schema.paramNames = append(schema.paramNames, pname)
		schema.constraints = append(schema.constraints, c.resolveConstraint(tp))
	}
	if td.Alias {
		if td.Type == nil {
			c.error(td.Name.Pos(), "missing alias type for %s", name)
		}
		// RHS validated under type-param env at instantiation (Tamil-0.76).
	} else if _, ok := td.Type.(*ast.StructType); ok {
		schema.isStruct = true
		schema.schematicType = c.registerSchematicStruct(td, schema)
	} else if td.Type == nil {
		c.error(td.Name.Pos(), "missing type body for generic type %s", name)
	}
	c.typeParamEnv = prevEnv
	c.cur.typeExp[name] = td.Exported
	c.cur.genericTypes[name] = schema
}

func (c *Checker) registerSchematicStruct(td *ast.TypeDecl, schema *genericTypeSchema) Type {
	stype, ok := td.Type.(*ast.StructType)
	if !ok {
		return TypeInvalid
	}
	tid := c.nextNamed
	if tid >= typeDefinedStart {
		c.error(td.Name.Pos(), "too many struct types")
		return TypeInvalid
	}
	c.nextNamed++
	name := td.Name.Name
	si := &StructInfo{
		Pkg:       c.cur.name,
		Name:      name,
		NamePos:   td.Name.Pos(),
		Methods:   map[string]*MethodInfo{},
		Schematic: true,
	}
	seen := map[string]bool{}
	for _, f := range stype.Fields {
		ft := c.typeFromExpr(f.Type)
		if ft == TypeInvalid || ft == TypeVoid {
			c.error(f.Type.Pos(), "invalid field type")
			ft = TypeInvalid
		} else if !c.isFieldType(ft) {
			c.error(f.Type.Pos(), "unsupported field type %s", c.typStr(ft))
			ft = TypeInvalid
		}
		if seen[f.Name.Name] {
			c.error(f.Name.Pos(), "duplicate field: %s", f.Name.Name)
		}
		seen[f.Name.Name] = true
		si.Fields = append(si.Fields, StructField{Name: f.Name.Name, Type: ft, Exported: f.Exported})
	}
	c.info.Structs[tid] = si
	c.info.TypeByName[c.cur.name+"."+name+"__schematic"] = tid
	return tid
}

func (c *Checker) lookupGenericSchema(pkg, name string) (*genericTypeSchema, *pkgState, bool) {
	st := c.cur
	if pkg != "" {
		if c.cur == nil {
			return nil, nil, false
		}
		imp, ok := c.cur.imports[pkg]
		if !ok {
			return nil, nil, false
		}
		st = imp
	}
	if st == nil {
		return nil, nil, false
	}
	sch, ok := st.genericTypes[name]
	return sch, st, ok
}

func (c *Checker) typeArgsFromExprs(exprs []ast.TypeExpr) []Type {
	out := make([]Type, len(exprs))
	for i, te := range exprs {
		out[i] = c.typeFromExpr(te)
	}
	return out
}

func (c *Checker) monoTypeKey(base string, typeArgs []Type) string {
	key := base
	for _, ta := range typeArgs {
		key += "__" + c.typStr(ta)
	}
	return key
}

func (c *Checker) copyGenericMethods(si *StructInfo, sch *genericTypeSchema, subst map[Type]Type, typeArgs []Type) {
	if si == nil || sch == nil || sch.schematicType == TypeInvalid {
		return
	}
	schematic, ok := c.info.Structs[sch.schematicType]
	if !ok || schematic.Methods == nil {
		return
	}
	si.GenericParamNames = append([]string(nil), sch.paramNames...)
	si.GenericTypeArgs = append([]Type(nil), typeArgs...)
	for mname, mi := range schematic.Methods {
		si.Methods[mname] = &MethodInfo{
			Name:      mi.Name,
			Exported:  mi.Exported,
			RecvIsPtr: mi.RecvIsPtr,
			RecvName:  mi.RecvName,
			Params:    c.substTypes(mi.Params, subst),
			Results:   c.substTypes(mi.Results, subst),
			Variadic:  mi.Variadic,
			Decl:      mi.Decl,
		}
	}
}

func (c *Checker) instantiateGenericType(sch *genericTypeSchema, st *pkgState, typeArgs []Type, pos token.Pos) Type {
	if sch == nil || sch.decl == nil || st == nil {
		return TypeInvalid
	}
	if len(typeArgs) != len(sch.typeParams) {
		c.error(pos, "wrong number of type arguments for %s (want %d, got %d)",
			sch.decl.Name.Name, len(sch.typeParams), len(typeArgs))
		return TypeInvalid
	}
	c.checkTypeArgsConstraints(typeArgs, sch.constraints, pos, sch.decl.Name.Name)

	key := st.name + "." + c.monoTypeKey(sch.decl.Name.Name, typeArgs)
	if t, ok := c.info.TypeByName[key]; ok {
		return t
	}

	subst := map[Type]Type{}
	for i, tp := range sch.typeParams {
		subst[tp] = typeArgs[i]
	}

	prevEnv := c.typeParamEnv
	c.typeParamEnv = map[string]Type{}
	for i, pname := range sch.paramNames {
		c.typeParamEnv[pname] = typeArgs[i]
	}

	var tid Type
	name := c.monoTypeKey(sch.decl.Name.Name, typeArgs)
	if sch.isAlias {
		// Resolve RHS with type parameters, then substitute concrete args.
		paramEnv := map[string]Type{}
		for i, pname := range sch.paramNames {
			paramEnv[pname] = sch.typeParams[i]
		}
		c.typeParamEnv = paramEnv
		raw := c.typeFromExpr(sch.decl.Type)
		under := c.substType(raw, subst)
		if under == TypeInvalid || under == TypeVoid {
			c.error(sch.decl.Type.Pos(), "invalid alias type")
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		c.typeParamEnv = prevEnv
		c.info.TypeByName[key] = under
		if st == c.cur {
			c.cur.types[name] = under
		}
		return under
	}
	if sch.isStruct {
		stype, ok := sch.decl.Type.(*ast.StructType)
		if !ok {
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		tid = c.nextNamed
		if tid >= typeDefinedStart {
			c.error(pos, "too many struct types")
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		c.nextNamed++
		si := &StructInfo{
			Pkg:     st.name,
			Name:    name,
			NamePos: sch.decl.Name.Pos(),
			Methods: map[string]*MethodInfo{},
		}
		seen := map[string]bool{}
		for _, f := range stype.Fields {
			ft := c.substType(c.typeFromExpr(f.Type), subst)
			if ft == TypeInvalid || ft == TypeVoid {
				c.error(f.Type.Pos(), "invalid field type")
				ft = TypeInvalid
			} else if !c.isFieldType(ft) {
				c.error(f.Type.Pos(), "unsupported field type %s", c.typStr(ft))
				ft = TypeInvalid
			}
			if seen[f.Name.Name] {
				c.error(f.Name.Pos(), "duplicate field: %s", f.Name.Name)
			}
			seen[f.Name.Name] = true
			si.Fields = append(si.Fields, StructField{Name: f.Name.Name, Type: ft, Exported: f.Exported})
		}
		c.copyGenericMethods(si, sch, subst, typeArgs)
		c.info.Structs[tid] = si
	} else {
		tid = c.nextDefined
		if tid >= typePointerStart {
			c.error(pos, "too many defined types")
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		c.nextDefined++
		under := c.substType(c.typeFromExpr(sch.decl.Type), subst)
		if under == TypeInvalid || under == TypeVoid {
			c.error(sch.decl.Type.Pos(), "invalid underlying type")
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		if isDefined(under) {
			c.error(sch.decl.Type.Pos(), "underlying type must not be another defined type")
			c.typeParamEnv = prevEnv
			return TypeInvalid
		}
		c.info.Defined[tid] = &DefinedInfo{Pkg: st.name, Name: name, NamePos: sch.decl.Name.Pos()}
		c.info.Underlying[tid] = under
	}

	c.typeParamEnv = prevEnv
	c.info.TypeByName[key] = tid
	if st == c.cur {
		c.cur.types[name] = tid
	}
	return tid
}

func (c *Checker) sameType(a, b Type) bool {
	if a == b {
		return true
	}
	return c.underlying(a) == c.underlying(b)
}

func (c *Checker) genericMethodTypeParamEnv(fn *ast.FuncDecl) map[string]Type {
	if fn == nil || fn.Recv == nil || c.cur == nil {
		return nil
	}
	recvType := fn.Recv.Type
	if pt, ok := recvType.(*ast.PointerType); ok {
		recvType = pt.Elem
	}
	tn, ok := recvType.(*ast.TypeName)
	if !ok || tn.Pkg != nil || len(tn.TypeArgs) == 0 {
		return nil
	}
	sch, _, ok := c.lookupGenericSchema("", tn.Name)
	if !ok || !c.typeArgsAreSchemaParams(tn.TypeArgs, sch) {
		return nil
	}
	env := map[string]Type{}
	for i, pname := range sch.paramNames {
		env[pname] = sch.typeParams[i]
	}
	return env
}

func (c *Checker) markGenericMethodTemplate(fn *ast.FuncDecl) {
	if fn == nil || c.info == nil {
		return
	}
	if c.info.GenericMethodTemplates == nil {
		c.info.GenericMethodTemplates = map[*ast.FuncDecl]bool{}
	}
	c.info.GenericMethodTemplates[fn] = true
}
