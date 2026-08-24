package check

import (
	"fmt"

	"niraluli/internal/ast"
	"niraluli/internal/token"
)

type pkgState struct {
	name       string
	file       *ast.File
	funcs      map[string]*funcSig
	types      map[string]Type
	typeExp    map[string]bool // name → வெளி
	aliases    map[string]bool // name → pending/resolved type alias
	imports    map[string]*pkgState
	importUsed map[string]bool // local import qualifier referenced
}

// CheckProgram type-checks merged package files in dependency order (deps first).
// Each ImportSpec.Pkg (real name) and Name (local qualifier) must be set by load.
func CheckProgram(merged []*ast.File, entry string) (*ProgramInfo, []error) {
	if len(merged) == 0 {
		return nil, []error{Error{Msg: "no source files"}}
	}
	if entry == "" {
		entry = "தொடக்கம்"
	}
	info := &Info{
		Types:         map[ast.Expr]Type{},
		Structs:       map[Type]*StructInfo{},
		Defined:       map[Type]*DefinedInfo{},
		Underlying:    map[Type]Type{},
		TypeByName:    map[string]Type{},
		PtrElem:       map[Type]Type{},
		SliceElem:     map[Type]Type{},
		TupleElems:    map[Type][]Type{},
		Arrays:        map[Type]ArrayInfo{},
		Maps:          map[Type]MapInfo{},
		Chans:         map[Type]ChanInfo{},
		Funcs:         map[Type]FuncInfo{},
		VariadicPacks: map[*ast.CallExpr]*VariadicPack{},
		TypeParamName: map[Type]string{},
		CallInst:      map[*ast.CallExpr]*MonoInst{},
		MethodValues:  map[ast.Expr]*MethodValueInfo{},
		MethodExprs:   map[ast.Expr]*MethodExprInfo{},
		PkgFuncValues: map[ast.Expr]*PkgFuncValueInfo{},
		Closures:      map[*ast.FuncLit]*ClosureInfo{},
		PromoteInFunc: map[*ast.FuncDecl]map[string]Type{},
		PromoteInLit:  map[*ast.FuncLit]map[string]Type{},
	}
	c := &Checker{
		scope:         &scope{vars: map[string]Type{}},
		info:          info,
		nextNamed:     typeNamedStart,
		nextDefined:   typeDefinedStart,
		nextPtr:       typePointerStart,
		nextSlice:     typeSliceStart,
		nextTuple:     typeTupleStart,
		nextArray:     typeArrayStart,
		nextMap:       typeMapStart,
		nextTypeParam: typeParamStart,
		nextFunc:      typeFuncStart,
		nextChan:      typeChanStart,
		pkgs:          map[string]*pkgState{},
	}
	for _, f := range merged {
		if f.Package == nil || f.Package.Name == nil {
			return nil, []error{Error{Msg: "missing package clause"}}
		}
		name := f.Package.Name.Name
		if _, ok := c.pkgs[name]; ok {
			return nil, []error{Error{Pos: f.Package.Name.Pos(), Msg: fmt.Sprintf("duplicate package %s", name)}}
		}
		c.pkgs[name] = &pkgState{
			name:       name,
			file:       f,
			funcs:      map[string]*funcSig{},
			types:      map[string]Type{},
			typeExp:    map[string]bool{},
			aliases:    map[string]bool{},
			imports:    map[string]*pkgState{},
			importUsed: map[string]bool{},
		}
	}
	for _, f := range merged {
		st := c.pkgs[f.Package.Name.Name]
		for _, im := range f.Imports {
			if im.Pkg == "" {
				c.error(im.Pos(), "import package name unresolved")
				continue
			}
			dep, ok := c.pkgs[im.Pkg]
			if !ok {
				c.error(im.Pos(), "unknown imported package %s", im.Pkg)
				continue
			}
			if im.Name == "_" {
				// blank import: load only
				continue
			}
			if im.Name == "" {
				c.error(im.Pos(), "import local name unresolved")
				continue
			}
			if prev, exists := st.imports[im.Name]; exists {
				if prev != dep {
					c.error(im.Pos(), "import name %s redeclared", im.Name)
				}
				continue
			}
			st.imports[im.Name] = dep
		}
	}
	for _, f := range merged {
		c.cur = c.pkgs[f.Package.Name.Name]
		for _, d := range f.Decls {
			if td, ok := d.(*ast.TypeDecl); ok {
				c.registerType(td)
			}
		}
	}
	for _, f := range merged {
		c.cur = c.pkgs[f.Package.Name.Name]
		c.resolveAliases(f.Decls)
		for _, d := range f.Decls {
			if td, ok := d.(*ast.TypeDecl); ok {
				c.fillTypeFields(td)
			}
		}
	}
	c.checkStructValueCycles()
	for _, f := range merged {
		c.cur = c.pkgs[f.Package.Name.Name]
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				c.collectFunc(d)
			case *ast.VarDecl:
				c.error(d.Pos(), "top-level variables not supported in Tamil-0")
			}
		}
	}
	for _, f := range merged {
		c.cur = c.pkgs[f.Package.Name.Name]
		c.isEntry = c.cur.name == entry
		c.checkPackageBody(f)
	}
	if est, ok := c.pkgs[entry]; ok {
		c.cur = est
		c.isEntry = true
		c.checkEntry()
	} else {
		c.error(token.Pos{}, "missing entry package %s", entry)
	}

	var pkgs []*PkgInfo
	for _, f := range merged {
		st := c.pkgs[f.Package.Name.Name]
		local := map[string]string{}
		for loc, dep := range st.imports {
			local[loc] = dep.name
		}
		pkgs = append(pkgs, &PkgInfo{Name: f.Package.Name.Name, File: f, ImportLocal: local})
	}
	return &ProgramInfo{Entry: entry, Pkgs: pkgs, Info: info}, c.errs
}
