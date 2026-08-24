// Package check type-checks Niraluli Tamil-0 ASTs.
package check

import (
	"fmt"

	"niraluli/internal/ast"
	"niraluli/internal/token"
)

// Type kinds for Tamil-0.3.
type Type int

const (
	TypeInvalid    Type = iota
	TypeInt             // முழுஎண்
	TypeBool            // நிலை
	TypeString          // சரம்
	TypeFloat           // மிதவைஎண்
	TypeByte            // இருமி8
	TypeRune            // இருமி32
	TypeVoid            // no result
	TypeSliceInt        // []முழுஎண்
	TypeSliceBool       // []நிலை
	TypeSliceStr        // []சரம்
	TypeSliceFloat      // []மிதவைஎண்
	TypeSliceByte       // []இருமி8
	TypeSliceRune       // []இருமி32
	TypeUntypedNil      // இன்மை before pointer context

	typeNamedStart   = 100   // user-defined named structs
	typeDefinedStart = 5000  // defined non-struct named types (வகை T U)
	typePointerStart = 10000 // interned pointer types (*T)
	typeSliceStart   = 20000 // interned nested slice types ([][]T, …)
	typeTupleStart   = 30000 // multi-value return tuples
	typeArrayStart   = 40000 // fixed arrays [N]T
	typeMapStart     = 50000 // maps அகராதி[K]V
	typeParamStart   = 60000 // function type parameters (generics)
	typeFuncStart    = 70000 // function types செயல்பாடு(…) (Tamil-0.44)
	typeChanStart    = 80000 // channels தடம் T (Tamil-0.48)
)

func (t Type) String() string {
	switch t {
	case TypeInt:
		return "முழுஎண்"
	case TypeBool:
		return "நிலை"
	case TypeString:
		return "சரம்"
	case TypeFloat:
		return "மிதவைஎண்"
	case TypeByte:
		return "இருமி8"
	case TypeRune:
		return "இருமி32"
	case TypeVoid:
		return "void"
	case TypeUntypedNil:
		return "இன்மை"
	case TypeSliceInt:
		return "[]முழுஎண்"
	case TypeSliceBool:
		return "[]நிலை"
	case TypeSliceStr:
		return "[]சரம்"
	case TypeSliceFloat:
		return "[]மிதவைஎண்"
	case TypeSliceByte:
		return "[]இருமி8"
	case TypeSliceRune:
		return "[]இருமி32"
	default:
		if t >= typeParamStart {
			return "வகைஅளவு"
		}
		if t >= typePointerStart {
			return "*T"
		}
		if t >= typeDefinedStart {
			return "வகை"
		}
		if t >= typeNamedStart {
			return "அமைப்பு"
		}
		return "invalid"
	}
}

// IsSlice reports whether t is a slice type (leaf or nested).
func IsSlice(t Type) bool {
	return t == TypeSliceInt || t == TypeSliceBool || t == TypeSliceStr || t == TypeSliceFloat ||
		t == TypeSliceByte || t == TypeSliceRune ||
		(t >= typeSliceStart && t < typeTupleStart)
}

// IsTuple reports whether t is a multi-value return tuple.
func IsTuple(t Type) bool {
	return t >= typeTupleStart && t < typeArrayStart
}

// IsArray reports whether t is a fixed array type [N]T.
func IsArray(t Type) bool {
	return t >= typeArrayStart && t < typeMapStart
}

// IsMap reports whether t is a map type அகராதி[K]V.
func IsMap(t Type) bool {
	return t >= typeMapStart && t < typeParamStart
}

// IsTypeParam reports whether t is a function type parameter.
func IsTypeParam(t Type) bool {
	return t >= typeParamStart && t < typeFuncStart
}

// IsFunc reports whether t is a function type செயல்பாடு(…)….
func IsFunc(t Type) bool {
	return t >= typeFuncStart && t < typeChanStart
}

// IsChan reports whether t is a channel type தடம் T.
func IsChan(t Type) bool {
	return t >= typeChanStart
}

// ArrayInfo describes a fixed array type.
type ArrayInfo struct {
	Len  int64
	Elem Type
}

// MapInfo describes a map type.
type MapInfo struct {
	Key  Type
	Elem Type
}

// ChanInfo describes a channel type.
type ChanInfo struct {
	Elem Type
	Dir  ast.ChanDir // 0 = bidirectional
}

// FuncInfo describes a function type செயல்பாடு(params) results.
type FuncInfo struct {
	Params   []Type
	Results  []Type // empty = void
	Variadic bool   // last Params entry is []T for ...T
}

// VariadicPack records how a call packs trailing args into a []T (Tamil-0.66).
type VariadicPack struct {
	Slice Type
	Elem  Type
	Fixed int // leading fixed argument count
}

// MethodValueInfo records a method value expression (X.M) for emit.
type MethodValueInfo struct {
	Method    *MethodInfo
	Struct    *StructInfo
	TakeAddr  bool // pointer method on addressable value → bind &X
	RecvIsPtr bool // X's type is already *T
}

// MethodExprInfo records a method expression T.M or (*T).M (Tamil-0.47).
type MethodExprInfo struct {
	Method      *MethodInfo
	Struct      *StructInfo
	ExprRecvPtr bool // true for (*T).M; false for T.M
	RecvType    Type // T or *T as written (first param of the func value)
}

// PkgFuncValueInfo records a package function used as a value.
type PkgFuncValueInfo struct {
	Pkg     string
	Name    string
	Params  []Type
	Results []Type
}

// CaptureVar is one free variable captured by a function literal.
type CaptureVar struct {
	Name string
	Type Type
}

// ClosureInfo records a function literal for emit (Tamil-0.45).
type ClosureInfo struct {
	Lit      *ast.FuncLit
	Params   []Type
	Results  []Type
	Variadic bool
	Captures []CaptureVar
	capSeen  map[string]bool
	ID       int
}

func isSlice(t Type) bool { return IsSlice(t) }

func isStruct(t Type) bool {
	return t >= typeNamedStart && t < typeDefinedStart
}

func isDefined(t Type) bool {
	return t >= typeDefinedStart && t < typePointerStart
}

func isPanicArgType(t Type) bool {
	switch t {
	case TypeString, TypeInt, TypeFloat, TypeBool, TypeByte, TypeRune:
		return true
	default:
		return false
	}
}

func isPointer(t Type) bool {
	return t >= typePointerStart && t < typeSliceStart
}

// ElemOfSlice returns the element type of a slice (leaf or nested).
func ElemOfSlice(info *Info, t Type) Type {
	switch t {
	case TypeSliceInt:
		return TypeInt
	case TypeSliceBool:
		return TypeBool
	case TypeSliceStr:
		return TypeString
	case TypeSliceFloat:
		return TypeFloat
	case TypeSliceByte:
		return TypeByte
	case TypeSliceRune:
		return TypeRune
	default:
		if info != nil {
			if e, ok := info.SliceElem[t]; ok {
				return e
			}
		}
		return TypeInvalid
	}
}

// StructField is one field of a named struct.
type StructField struct {
	Name     string
	Type     Type
	Exported bool
}

// MethodInfo describes a method on a named struct.
type MethodInfo struct {
	Name      string
	Exported  bool
	RecvIsPtr bool
	RecvName  string
	Params    []Type
	Results   []Type
	Variadic  bool
	Decl      *ast.FuncDecl
}

// StructInfo describes a named struct type.
type StructInfo struct {
	Pkg     string // declaring package name
	Name    string
	NamePos token.Pos
	Fields  []StructField
	Methods map[string]*MethodInfo
}

// DefinedInfo describes a non-struct named type (வகை T U).
type DefinedInfo struct {
	Pkg     string
	Name    string
	NamePos token.Pos
}

// Info holds type information from checking.
type Info struct {
	Types          map[ast.Expr]Type
	Structs        map[Type]*StructInfo
	Defined        map[Type]*DefinedInfo // defined non-struct named types
	Underlying     map[Type]Type         // defined type → underlying
	TypeByName     map[string]Type       // key: "Name" or "pkg.Name" within current check; emit uses Structs
	PtrElem        map[Type]Type         // pointer type → element type
	SliceElem      map[Type]Type         // non-leaf slice type → element (nested / struct / pointer)
	TupleElems     map[Type][]Type       // multi-value return type → component types
	Arrays         map[Type]ArrayInfo    // fixed array type → len + elem
	Maps           map[Type]MapInfo      // map type → key + elem
	Chans          map[Type]ChanInfo     // channel type → elem + dir
	Funcs          map[Type]FuncInfo     // function type → params/results
	VariadicPacks  map[*ast.CallExpr]*VariadicPack
	TypeParamName  map[Type]string       // type parameter → source name
	Instantiations []*MonoInst           // unique generic function instantiations
	CallInst       map[*ast.CallExpr]*MonoInst
	MethodValues   map[ast.Expr]*MethodValueInfo
	MethodExprs    map[ast.Expr]*MethodExprInfo
	PkgFuncValues  map[ast.Expr]*PkgFuncValueInfo
	Closures       map[*ast.FuncLit]*ClosureInfo
	// Locals/params that must be arena-promoted because a nested closure captures them.
	PromoteInFunc map[*ast.FuncDecl]map[string]Type
	PromoteInLit  map[*ast.FuncLit]map[string]Type
}

// MonoInst is one monomorphized instantiation of a generic function.
type MonoInst struct {
	Pkg      string
	Name     string
	Decl     *ast.FuncDecl
	TypeArgs []Type
	Params   []Type
	Results  []Type
	Key      string // stable suffix for C naming
}

// PkgInfo is one type-checked package for emission.
type PkgInfo struct {
	Name        string
	File        *ast.File         // merged
	ImportLocal map[string]string // local qualifier → real package name
}

// ProgramInfo is a checked multi-package program.
type ProgramInfo struct {
	Entry string
	Pkgs  []*PkgInfo // dependencies first
	Info  *Info
}

// Error is a type-check error.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

type funcSig struct {
	params         []Type
	results        []Type // empty = void
	variadic       bool
	exported       bool
	decl           *ast.FuncDecl
	typeParams     []Type // schematic type-parameter ids (ordered)
	typeParamNames []string
}

func (s *funcSig) generic() bool {
	return s != nil && len(s.typeParams) > 0
}

func (s *funcSig) resultType(c *Checker) Type {
	if s == nil {
		return TypeVoid
	}
	return c.typeFromResultList(s.results)
}

type scope struct {
	parent *scope
	vars   map[string]Type
}

func (s *scope) lookup(name string) (Type, bool) {
	t, _, ok := s.lookupScope(name)
	return t, ok
}

func (s *scope) lookupScope(name string) (Type, *scope, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if t, ok := cur.vars[name]; ok {
			return t, cur, true
		}
	}
	return TypeInvalid, nil, false
}

func (s *scope) declare(name string, t Type, pos token.Pos, errs *[]error) {
	if _, exists := s.vars[name]; exists {
		*errs = append(*errs, Error{Pos: pos, Msg: fmt.Sprintf("already declared: %s", name)})
		return
	}
	s.vars[name] = t
}

type litFrame struct {
	lit  *ast.FuncLit
	root *scope
	info *ClosureInfo
}

// Checker holds check state.
type Checker struct {
	errs          []error
	scope         *scope
	curFn         *funcSig
	enclosingFn   *ast.FuncDecl
	litStack      []litFrame
	nextClosure   int
	loopDepth     int
	info          *Info
	nextNamed     Type
	nextDefined   Type
	nextPtr       Type
	nextSlice     Type
	nextTuple     Type
	nextArray     Type
	nextMap       Type
	nextTypeParam Type
	nextFunc      Type
	nextChan      Type
	typeParamEnv  map[string]Type // active while checking a generic function
	allowMulti    bool            // true while unpacking a multi-value call
	discardMulti  bool            // true while a statement discards a call's results
	pkgs          map[string]*pkgState
	cur           *pkgState
	isEntry       bool
}

// File type-checks a single source file (as the entry package).
func File(f *ast.File) (*Info, []error) {
	_, info, errs := Package([]*ast.File{f})
	return info, errs
}

// Package type-checks one or more same-package files as the entry package.
// Returns a merged File suitable for emitc.Emit (single-package programs).
func Package(files []*ast.File) (*ast.File, *Info, []error) {
	merged, merrs := mergePackage(files)
	if len(merrs) != 0 {
		return nil, nil, merrs
	}
	pi, errs := CheckProgram([]*ast.File{merged}, merged.Package.Name.Name)
	if pi == nil {
		return merged, nil, errs
	}
	return merged, pi.Info, errs
}

func mergePackage(files []*ast.File) (*ast.File, []error) {
	if len(files) == 0 {
		return nil, []error{Error{Msg: "no source files"}}
	}
	var errs []error
	var pkg *ast.PackageClause
	var imports []*ast.ImportSpec
	var decls []ast.Decl
	path := ""
	for i, f := range files {
		if f == nil {
			errs = append(errs, Error{Msg: fmt.Sprintf("nil file at index %d", i)})
			continue
		}
		if f.Package == nil || f.Package.Name == nil {
			errs = append(errs, Error{Msg: "missing package clause"})
			continue
		}
		if pkg == nil {
			pkg = f.Package
		} else if f.Package.Name.Name != pkg.Name.Name {
			errs = append(errs, Error{
				Pos: f.Package.Name.Pos(),
				Msg: fmt.Sprintf("package name mismatch: got %s, want %s", f.Package.Name.Name, pkg.Name.Name),
			})
		}
		if path == "" {
			path = f.Path
		}
		imports = append(imports, f.Imports...)
		decls = append(decls, f.Decls...)
	}
	if len(errs) != 0 {
		return nil, errs
	}
	if pkg == nil {
		return nil, []error{Error{Msg: "missing package clause"}}
	}
	return &ast.File{Path: path, Package: pkg, Imports: imports, Decls: decls}, nil
}

func (c *Checker) checkPackageBody(f *ast.File) {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			c.checkFunc(fn)
		}
	}
	if c.cur == nil {
		return
	}
	for _, im := range f.Imports {
		if im.Name == "" || im.Name == "_" {
			continue
		}
		if !c.cur.importUsed[im.Name] {
			c.error(im.Pos(), "imported and not used: %s", im.Name)
		}
	}
}

func (c *Checker) markImportUsed(local string) {
	if c.cur != nil && local != "" {
		c.cur.importUsed[local] = true
	}
}

func (c *Checker) checkEntry() {
	if c.cur == nil {
		return
	}
	if c.cur.name != "தொடக்கம்" {
		c.error(c.cur.file.Package.Name.Pos(), "entry package name must be தொடக்கம்")
	}
	entry, ok := c.cur.funcs["தொடக்கம்"]
	if !ok {
		c.error(c.cur.file.Package.Pos(), "missing entry function தொடக்கம்")
	} else if len(entry.params) != 0 || len(entry.results) != 0 {
		c.error(entry.decl.Name.Pos(), "தொடக்கம் must have no parameters and no result type")
	}
}

func (c *Checker) typesFromResults(rs []*ast.Field) []Type {
	if len(rs) == 0 {
		return nil
	}
	out := make([]Type, len(rs))
	for i, r := range rs {
		out[i] = c.typeFromExpr(r.Type)
	}
	return out
}

func (c *Checker) typeFromResultList(rs []Type) Type {
	switch len(rs) {
	case 0:
		return TypeVoid
	case 1:
		return rs[0]
	default:
		return c.tupleOf(rs)
	}
}

func (c *Checker) tupleOf(elems []Type) Type {
	for t, e := range c.info.TupleElems {
		if len(e) != len(elems) {
			continue
		}
		same := true
		for i := range elems {
			if e[i] != elems[i] {
				same = false
				break
			}
		}
		if same {
			return t
		}
	}
	t := c.nextTuple
	c.nextTuple++
	cp := make([]Type, len(elems))
	copy(cp, elems)
	c.info.TupleElems[t] = cp
	return t
}

func (c *Checker) arrayOf(n int64, elem Type) Type {
	for t, ai := range c.info.Arrays {
		if ai.Len == n && ai.Elem == elem {
			return t
		}
	}
	t := c.nextArray
	c.nextArray++
	c.info.Arrays[t] = ArrayInfo{Len: n, Elem: elem}
	return t
}

// mapKeyOK reports whether t may be used as a map key (Go: comparable).
func (c *Checker) mapKeyOK(t Type) bool {
	return c.comparable(t)
}

func (c *Checker) mapOf(key, elem Type) (Type, bool) {
	if key == TypeInvalid || elem == TypeInvalid || elem == TypeVoid {
		return TypeInvalid, false
	}
	if !c.mapKeyOK(key) {
		return TypeInvalid, false
	}
	if !c.isFieldType(elem) && !IsMap(elem) {
		return TypeInvalid, false
	}
	for t, mi := range c.info.Maps {
		if mi.Key == key && mi.Elem == elem {
			return t, true
		}
	}
	t := c.nextMap
	c.nextMap++
	c.info.Maps[t] = MapInfo{Key: key, Elem: elem}
	// Pre-intern (value, bool) for comma-ok indexing.
	c.tupleOf([]Type{elem, TypeBool})
	return t, true
}

func (c *Checker) chanOf(elem Type, dir ast.ChanDir) Type {
	if elem == TypeInvalid || elem == TypeVoid {
		return TypeInvalid
	}
	if !c.isFieldType(elem) && !IsChan(elem) {
		return TypeInvalid
	}
	for t, ci := range c.info.Chans {
		if ci.Elem == elem && ci.Dir == dir {
			return t
		}
	}
	t := c.nextChan
	c.nextChan++
	c.info.Chans[t] = ChanInfo{Elem: elem, Dir: dir}
	c.tupleOf([]Type{elem, TypeBool})
	return t
}

func typesEqual(a, b []Type) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *Checker) funcOf(params, results []Type, variadic bool) Type {
	for t, fi := range c.info.Funcs {
		if fi.Variadic == variadic && typesEqual(fi.Params, params) && typesEqual(fi.Results, results) {
			return t
		}
	}
	t := c.nextFunc
	c.nextFunc++
	pc := append([]Type(nil), params...)
	rc := append([]Type(nil), results...)
	c.info.Funcs[t] = FuncInfo{Params: pc, Results: rc, Variadic: variadic}
	return t
}

func (c *Checker) typStr(t Type) string {
	if IsFunc(t) {
		fi := c.info.Funcs[t]
		s := "செயல்பாடு("
		for i, p := range fi.Params {
			if i > 0 {
				s += ", "
			}
			if fi.Variadic && i == len(fi.Params)-1 {
				s += "..." + c.typStr(ElemOfSlice(c.info, p))
			} else {
				s += c.typStr(p)
			}
		}
		s += ")"
		switch len(fi.Results) {
		case 0:
		case 1:
			s += " " + c.typStr(fi.Results[0])
		default:
			s += " ("
			for i, r := range fi.Results {
				if i > 0 {
					s += ", "
				}
				s += c.typStr(r)
			}
			s += ")"
		}
		return s
	}
	if IsMap(t) {
		mi := c.info.Maps[t]
		return "அகராதி[" + c.typStr(mi.Key) + "]" + c.typStr(mi.Elem)
	}
	if IsChan(t) {
		ci := c.info.Chans[t]
		switch ci.Dir {
		case ast.SEND:
			return "தடம்<-" + c.typStr(ci.Elem)
		case ast.RECV:
			return "<-தடம்" + c.typStr(ci.Elem)
		default:
			return "தடம் " + c.typStr(ci.Elem)
		}
	}
	if IsArray(t) {
		ai := c.info.Arrays[t]
		return fmt.Sprintf("[%d]%s", ai.Len, c.typStr(ai.Elem))
	}
	if IsTuple(t) {
		elems := c.info.TupleElems[t]
		s := "("
		for i, e := range elems {
			if i > 0 {
				s += ", "
			}
			s += c.typStr(e)
		}
		return s + ")"
	}
	if isSlice(t) {
		return "[]" + c.typStr(ElemOfSlice(c.info, t))
	}
	if isPointer(t) {
		if elem, ok := c.info.PtrElem[t]; ok {
			return "*" + c.typStr(elem)
		}
		return "*T"
	}
	if si, ok := c.info.Structs[t]; ok {
		return si.Name
	}
	if di, ok := c.info.Defined[t]; ok {
		return di.Name
	}
	if name, ok := c.info.TypeParamName[t]; ok {
		return name
	}
	return t.String()
}

func (c *Checker) allocTypeParam(name string) Type {
	t := c.nextTypeParam
	c.nextTypeParam++
	c.info.TypeParamName[t] = name
	return t
}

func (c *Checker) sliceOf(elem Type) (Type, bool) {
	switch elem {
	case TypeInt:
		return TypeSliceInt, true
	case TypeBool:
		return TypeSliceBool, true
	case TypeString:
		return TypeSliceStr, true
	case TypeFloat:
		return TypeSliceFloat, true
	case TypeByte:
		return TypeSliceByte, true
	case TypeRune:
		return TypeSliceRune, true
	}
	// Nested slices, []struct, []*T, []defined, []func, []type-param.
	if !isSlice(elem) && !isStruct(elem) && !isPointer(elem) && !isDefined(elem) && !IsFunc(elem) && !IsTypeParam(elem) {
		return TypeInvalid, false
	}
	for st, e := range c.info.SliceElem {
		if e == elem {
			return st, true
		}
	}
	st := c.nextSlice
	c.nextSlice++
	c.info.SliceElem[st] = elem
	return st, true
}

func (c *Checker) pointerOf(elem Type) Type {
	if elem == TypeInvalid || elem == TypeVoid {
		return TypeInvalid
	}
	for pt, e := range c.info.PtrElem {
		if e == elem {
			return pt
		}
	}
	pt := c.nextPtr
	c.nextPtr++
	c.info.PtrElem[pt] = elem
	return pt
}

func (c *Checker) elemOfPtr(t Type) Type {
	if e, ok := c.info.PtrElem[t]; ok {
		return e
	}
	return TypeInvalid
}

func (c *Checker) record(e ast.Expr, t Type) Type {
	if e != nil && c.info != nil {
		c.info.Types[e] = t
	}
	return t
}

func (c *Checker) error(pos token.Pos, format string, args ...any) {
	c.errs = append(c.errs, Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// registerType allocates a named type (struct / defined; aliases filled later).
func (c *Checker) registerType(td *ast.TypeDecl) {
	if td.Name == nil || c.cur == nil {
		return
	}
	name := td.Name.Name
	if _, exists := c.cur.types[name]; exists {
		c.error(td.Name.Pos(), "type redeclared: %s", name)
		return
	}
	c.cur.typeExp[name] = td.Exported
	if td.Alias {
		// Placeholder until fill resolves the aliased type.
		c.cur.types[name] = TypeInvalid
		c.cur.aliases[name] = true
		return
	}
	if _, ok := td.Type.(*ast.StructType); ok {
		tid := c.nextNamed
		if tid >= typeDefinedStart {
			c.error(td.Name.Pos(), "too many struct types")
			return
		}
		c.nextNamed++
		c.cur.types[name] = tid
		c.info.TypeByName[c.cur.name+"."+name] = tid
		c.info.Structs[tid] = &StructInfo{
			Pkg:     c.cur.name,
			Name:    name,
			NamePos: td.Name.Pos(),
			Methods: map[string]*MethodInfo{},
		}
		return
	}
	// Defined non-struct type: வகை T U
	tid := c.nextDefined
	if tid >= typePointerStart {
		c.error(td.Name.Pos(), "too many defined types")
		return
	}
	c.nextDefined++
	c.cur.types[name] = tid
	c.info.TypeByName[c.cur.name+"."+name] = tid
	c.info.Defined[tid] = &DefinedInfo{Pkg: c.cur.name, Name: name, NamePos: td.Name.Pos()}
	c.info.Underlying[tid] = TypeInvalid // filled later
}

// fillTypeFields resolves struct fields and defined/alias underlying types.
func (c *Checker) fillTypeFields(td *ast.TypeDecl) {
	if td.Name == nil || c.cur == nil {
		return
	}
	name := td.Name.Name
	tid, ok := c.cur.types[name]
	if !ok {
		return
	}
	if td.Alias {
		return // resolved in resolveAliases
	}
	if st, ok := td.Type.(*ast.StructType); ok {
		si := c.info.Structs[tid]
		seen := map[string]bool{}
		for _, f := range st.Fields {
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
		return
	}
	// Defined type underlying.
	under := c.typeFromExpr(td.Type)
	if under == TypeInvalid || under == TypeVoid {
		c.error(td.Type.Pos(), "invalid underlying type")
		return
	}
	if isDefined(under) {
		c.error(td.Type.Pos(), "underlying type must not be another defined type")
		return
	}
	c.info.Underlying[tid] = under
}

// resolveAliases binds வகை T = U names after structs/defined are registered.
func (c *Checker) resolveAliases(decls []ast.Decl) {
	pending := []*ast.TypeDecl{}
	for _, d := range decls {
		if td, ok := d.(*ast.TypeDecl); ok && td.Alias {
			pending = append(pending, td)
		}
	}
	for len(pending) > 0 {
		progress := false
		var next []*ast.TypeDecl
		for _, td := range pending {
			name := td.Name.Name
			under := c.typeFromExpr(td.Type)
			if under == TypeInvalid {
				// May still be an unresolved alias placeholder.
				if tn, ok := td.Type.(*ast.TypeName); ok && tn.Pkg == nil && c.cur.aliases[tn.Name] {
					if c.cur.types[tn.Name] == TypeInvalid {
						next = append(next, td)
						continue
					}
					under = c.cur.types[tn.Name]
				}
			}
			if under == TypeInvalid || under == TypeVoid {
				c.error(td.Type.Pos(), "invalid alias type")
				delete(c.cur.aliases, name)
				progress = true
				continue
			}
			c.cur.types[name] = under
			c.info.TypeByName[c.cur.name+"."+name] = under
			delete(c.cur.aliases, name)
			progress = true
		}
		if !progress {
			for _, td := range next {
				c.error(td.Type.Pos(), "invalid alias type")
				delete(c.cur.aliases, td.Name.Name)
			}
			break
		}
		pending = next
	}
}

func (c *Checker) isFieldType(t Type) bool {
	if t == TypeInt || t == TypeBool || t == TypeString || t == TypeFloat || t == TypeByte || t == TypeRune {
		return true
	}
	if isSlice(t) || IsArray(t) || isPointer(t) || isStruct(t) || IsMap(t) || IsFunc(t) || IsChan(t) {
		return true
	}
	if isDefined(t) {
		return c.isFieldType(c.info.Underlying[t])
	}
	return false
}

// comparable reports whether == / != is allowed (Go-like).
func (c *Checker) comparable(t Type) bool {
	switch {
	case t == TypeInt || t == TypeBool || t == TypeString || t == TypeFloat || t == TypeByte || t == TypeRune:
		return true
	case isPointer(t):
		return true
	case IsChan(t):
		return true
	case isSlice(t), IsMap(t):
		return false
	case IsArray(t):
		ai := c.info.Arrays[t]
		return c.comparable(ai.Elem)
	case isDefined(t):
		return c.comparable(c.info.Underlying[t])
	case isStruct(t):
		si := c.info.Structs[t]
		if si == nil {
			return false
		}
		for _, f := range si.Fields {
			if f.Type == TypeInvalid || !c.comparable(f.Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// checkStructValueCycles rejects T containing T (or a cycle) by value.
func (c *Checker) checkStructValueCycles() {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[Type]int{}
	var visit func(Type, token.Pos, string) bool
	visit = func(t Type, pos token.Pos, name string) bool {
		switch color[t] {
		case gray:
			c.error(pos, "invalid recursive type %s (use *%s for a pointer field)", name, name)
			return false
		case black:
			return true
		}
		color[t] = gray
		si := c.info.Structs[t]
		for _, f := range si.Fields {
			if isStruct(f.Type) {
				inner := c.info.Structs[f.Type]
				if inner == nil {
					continue
				}
				if !visit(f.Type, pos, name) {
					return false
				}
			}
		}
		color[t] = black
		return true
	}
	for t, si := range c.info.Structs {
		if color[t] == white {
			visit(t, si.NamePos, si.Name)
		}
	}
}

func (c *Checker) typeFromExpr(te ast.TypeExpr) Type {
	if te == nil {
		return TypeVoid
	}
	switch te := te.(type) {
	case *ast.TypeName:
		return c.typeFromName(te)
	case *ast.SliceType:
		elem := c.typeFromExpr(te.Elem)
		st, ok := c.sliceOf(elem)
		if !ok {
			return TypeInvalid
		}
		return st
	case *ast.ArrayType:
		if te.Len < 0 {
			return TypeInvalid
		}
		elem := c.typeFromExpr(te.Elem)
		if elem == TypeInvalid || elem == TypeVoid {
			return TypeInvalid
		}
		// Element must be a leaf scalar, slice, array, pointer, struct, or defined.
		if elem != TypeInt && elem != TypeBool && elem != TypeString && elem != TypeFloat &&
			elem != TypeByte && elem != TypeRune &&
			!isSlice(elem) && !IsArray(elem) && !isPointer(elem) && !isStruct(elem) && !isDefined(elem) &&
			!IsTypeParam(elem) {
			return TypeInvalid
		}
		return c.arrayOf(te.Len, elem)
	case *ast.PointerType:
		elem := c.typeFromExpr(te.Elem)
		if elem == TypeInvalid {
			return TypeInvalid
		}
		return c.pointerOf(elem)
	case *ast.MapType:
		key := c.typeFromExpr(te.Key)
		elem := c.typeFromExpr(te.Elem)
		if key == TypeInvalid || elem == TypeInvalid {
			return TypeInvalid
		}
		if !c.mapKeyOK(key) {
			c.error(te.Key.Pos(), "invalid map key type %s (must be comparable)", c.typStr(key))
			return TypeInvalid
		}
		mt, ok := c.mapOf(key, elem)
		if !ok {
			c.error(te.Pos(), "invalid map type அகராதி[%s]%s", c.typStr(key), c.typStr(elem))
			return TypeInvalid
		}
		return mt
	case *ast.ChanType:
		elem := c.typeFromExpr(te.Elem)
		if elem == TypeInvalid || elem == TypeVoid {
			return TypeInvalid
		}
		ct := c.chanOf(elem, te.Dir)
		if ct == TypeInvalid {
			c.error(te.Pos(), "invalid channel element type %s", c.typStr(elem))
			return TypeInvalid
		}
		return ct
	case *ast.FuncType:
		params := make([]Type, len(te.Params))
		for i, p := range te.Params {
			params[i] = c.typeFromExpr(p)
			if params[i] == TypeInvalid || params[i] == TypeVoid {
				return TypeInvalid
			}
		}
		if te.Variadic {
			if len(params) == 0 {
				return TypeInvalid
			}
			last := len(params) - 1
			st, ok := c.sliceOf(params[last])
			if !ok {
				c.error(te.Pos(), "invalid variadic element type %s", c.typStr(params[last]))
				return TypeInvalid
			}
			params[last] = st
		}
		results := c.typesFromResults(te.Results)
		for _, r := range results {
			if r == TypeInvalid {
				return TypeInvalid
			}
		}
		return c.funcOf(params, results, te.Variadic)
	default:
		return TypeInvalid
	}
}

func (c *Checker) typeFromName(tn *ast.TypeName) Type {
	if tn == nil {
		return TypeVoid
	}
	if tn.Pkg != nil {
		if c.cur == nil {
			return TypeInvalid
		}
		imp, ok := c.cur.imports[tn.Pkg.Name]
		if !ok {
			c.error(tn.Pkg.Pos(), "unknown package %s", tn.Pkg.Name)
			return TypeInvalid
		}
		c.markImportUsed(tn.Pkg.Name)
		if t, ok := imp.types[tn.Name]; ok {
			if !imp.typeExp[tn.Name] {
				c.error(tn.TokPos, "type %s.%s is not exported (need வெளி)", tn.Pkg.Name, tn.Name)
				return TypeInvalid
			}
			return t
		}
		c.error(tn.TokPos, "package %s has no type %s", tn.Pkg.Name, tn.Name)
		return TypeInvalid
	}
	switch tn.Name {
	case "முழுஎண்":
		return TypeInt
	case "நிலை":
		return TypeBool
	case "சரம்":
		return TypeString
	case "மிதவைஎண்":
		return TypeFloat
	case "இருமி8":
		return TypeByte
	case "இருமி32":
		return TypeRune
	default:
		if c.typeParamEnv != nil {
			if t, ok := c.typeParamEnv[tn.Name]; ok {
				return t
			}
		}
		if c.cur != nil {
			if t, ok := c.cur.types[tn.Name]; ok {
				return t
			}
		}
		return TypeInvalid
	}
}

func (c *Checker) paramsFromFields(fields []*ast.Field) ([]Type, bool) {
	params := make([]Type, 0, len(fields))
	variadic := false
	for i, p := range fields {
		if p == nil {
			params = append(params, TypeInvalid)
			continue
		}
		t := c.typeFromExpr(p.Type)
		if p.Ellipsis {
			if i != len(fields)-1 {
				c.error(p.Type.Pos(), "... is only allowed on the final parameter")
			}
			st, ok := c.sliceOf(t)
			if !ok {
				c.error(p.Type.Pos(), "invalid variadic element type %s", c.typStr(t))
				t = TypeInvalid
			} else {
				t = st
			}
			variadic = true
		}
		params = append(params, t)
	}
	return params, variadic
}

func (c *Checker) checkCallArgs(e *ast.CallExpr, params []Type, variadic bool, label string) {
	if e == nil {
		return
	}
	if !variadic {
		if len(e.Args) != len(params) {
			c.error(e.Pos(), "wrong number of arguments to %s (want %d, got %d)", label, len(params), len(e.Args))
		}
		for i, arg := range e.Args {
			t := c.checkExpr(arg)
			if i < len(params) && !c.assignable(t, params[i], arg) {
				c.error(arg.Pos(), "argument %d: want %s, got %s", i+1, c.typStr(params[i]), c.typStr(t))
			}
		}
		return
	}
	if len(params) == 0 {
		c.error(e.Pos(), "invalid variadic signature for %s", label)
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return
	}
	fixed := len(params) - 1
	if len(e.Args) < fixed {
		c.error(e.Pos(), "wrong number of arguments to %s (want at least %d, got %d)", label, fixed, len(e.Args))
	}
	for i := 0; i < fixed && i < len(e.Args); i++ {
		t := c.checkExpr(e.Args[i])
		if !c.assignable(t, params[i], e.Args[i]) {
			c.error(e.Args[i].Pos(), "argument %d: want %s, got %s", i+1, c.typStr(params[i]), c.typStr(t))
		}
	}
	elem := ElemOfSlice(c.info, params[fixed])
	for i := fixed; i < len(e.Args); i++ {
		t := c.checkExpr(e.Args[i])
		if !c.assignable(t, elem, e.Args[i]) {
			c.error(e.Args[i].Pos(), "argument %d: want %s, got %s", i+1, c.typStr(elem), c.typStr(t))
		}
	}
	c.info.VariadicPacks[e] = &VariadicPack{Slice: params[fixed], Elem: elem, Fixed: fixed}
}

func (c *Checker) collectFunc(fn *ast.FuncDecl) {
	if fn.Name == nil || c.cur == nil {
		return
	}
	name := fn.Name.Name
	prevEnv := c.typeParamEnv
	if len(fn.TypeParams) > 0 {
		if fn.Recv != nil {
			c.error(fn.Name.Pos(), "type parameters not allowed on methods")
			return
		}
		if name == "தொடக்கம்" {
			c.error(fn.Name.Pos(), "entry function தொடக்கம் cannot be generic")
			return
		}
		c.typeParamEnv = map[string]Type{}
		seen := map[string]bool{}
		for _, tp := range fn.TypeParams {
			if tp == nil || tp.Name == "" {
				continue
			}
			if seen[tp.Name] {
				c.error(tp.Pos(), "duplicate type parameter %s", tp.Name)
				continue
			}
			seen[tp.Name] = true
			t := c.allocTypeParam(tp.Name)
			c.typeParamEnv[tp.Name] = t
		}
	}
	sig := &funcSig{results: c.typesFromResults(fn.Results), decl: fn}
	sig.params, sig.variadic = c.paramsFromFields(fn.Params)
	if len(fn.TypeParams) > 0 {
		for _, tp := range fn.TypeParams {
			if tp == nil {
				continue
			}
			if t, ok := c.typeParamEnv[tp.Name]; ok {
				sig.typeParams = append(sig.typeParams, t)
				sig.typeParamNames = append(sig.typeParamNames, tp.Name)
			}
		}
	}
	c.typeParamEnv = prevEnv

	if fn.Recv != nil {
		c.collectMethod(fn, sig)
		return
	}
	if _, exists := c.cur.funcs[name]; exists {
		c.error(fn.Name.Pos(), "function redeclared: %s", name)
		return
	}
	sig.exported = fn.Exported
	c.cur.funcs[name] = sig
}

func (c *Checker) collectMethod(fn *ast.FuncDecl, sig *funcSig) {
	rt := c.typeFromExpr(fn.Recv.Type)
	recvIsPtr := false
	base := rt
	if isPointer(rt) {
		recvIsPtr = true
		base = c.elemOfPtr(rt)
	}
	if !isStruct(base) {
		c.error(fn.Recv.Type.Pos(), "method receiver must be a named struct or *struct")
		return
	}
	si := c.info.Structs[base]
	if si.Methods == nil {
		si.Methods = map[string]*MethodInfo{}
	}
	mname := fn.Name.Name
	for _, f := range si.Fields {
		if f.Name == mname {
			c.error(fn.Name.Pos(), "method %s conflicts with field %s", mname, f.Name)
			return
		}
	}
	if _, exists := si.Methods[mname]; exists {
		c.error(fn.Name.Pos(), "method redeclared: %s.%s", si.Name, mname)
		return
	}
	si.Methods[mname] = &MethodInfo{
		Name:      mname,
		Exported:  fn.Exported,
		RecvIsPtr: recvIsPtr,
		RecvName:  fn.Recv.Name.Name,
		Params:    sig.params,
		Results:   sig.results,
		Variadic:  sig.variadic,
		Decl:      fn,
	}
}

func (c *Checker) push() {
	c.scope = &scope{parent: c.scope, vars: map[string]Type{}}
}

func (c *Checker) pop() {
	c.scope = c.scope.parent
}

func (c *Checker) checkFunc(fn *ast.FuncDecl) {
	var sig *funcSig
	if fn.Recv != nil {
		rt := c.typeFromExpr(fn.Recv.Type)
		base := rt
		if isPointer(rt) {
			base = c.elemOfPtr(rt)
		}
		if si, ok := c.info.Structs[base]; ok {
			if mi, ok := si.Methods[fn.Name.Name]; ok {
				sig = &funcSig{params: mi.Params, results: mi.Results, variadic: mi.Variadic, decl: fn}
			}
		}
		if sig == nil {
			sig = &funcSig{results: c.typesFromResults(fn.Results), decl: fn}
			sig.params, sig.variadic = c.paramsFromFields(fn.Params)
		}
	} else if c.cur != nil {
		sig = c.cur.funcs[fn.Name.Name]
	}
	c.curFn = sig
	prevEncl := c.enclosingFn
	c.enclosingFn = fn
	prevEnv := c.typeParamEnv
	if sig != nil && sig.generic() {
		c.typeParamEnv = map[string]Type{}
		for i, name := range sig.typeParamNames {
			c.typeParamEnv[name] = sig.typeParams[i]
		}
	}
	c.push()
	if fn.Recv != nil {
		rt := c.typeFromExpr(fn.Recv.Type)
		if rt == TypeInvalid {
			c.error(fn.Recv.Type.Pos(), "invalid receiver type")
		}
		c.scope.declare(fn.Recv.Name.Name, rt, fn.Recv.Name.Pos(), &c.errs)
	}
	if sig != nil {
		for i, p := range fn.Params {
			t := TypeInvalid
			if i < len(sig.params) {
				t = sig.params[i]
			}
			if t == TypeInvalid {
				c.error(p.Type.Pos(), "invalid parameter type")
			}
			c.scope.declare(p.Name.Name, t, p.Name.Pos(), &c.errs)
		}
	} else {
		for _, p := range fn.Params {
			t := c.typeFromExpr(p.Type)
			if p.Ellipsis {
				if st, ok := c.sliceOf(t); ok {
					t = st
				}
			}
			if t == TypeInvalid {
				c.error(p.Type.Pos(), "invalid parameter type")
			}
			c.scope.declare(p.Name.Name, t, p.Name.Pos(), &c.errs)
		}
	}
	for _, r := range fn.Results {
		if r.Name == nil {
			continue
		}
		t := c.typeFromExpr(r.Type)
		if t == TypeInvalid {
			c.error(r.Type.Pos(), "invalid result type")
		}
		c.scope.declare(r.Name.Name, t, r.Name.Pos(), &c.errs)
	}
	c.checkBlock(fn.Body)
	c.pop()
	c.typeParamEnv = prevEnv
	c.enclosingFn = prevEncl
	c.curFn = nil
}

func scopeContains(ancestor, s *scope) bool {
	for cur := s; cur != nil; cur = cur.parent {
		if cur == ancestor {
			return true
		}
	}
	return false
}

func (c *Checker) noteCapture(name string, t Type, declScope *scope) {
	if t == TypeInvalid || t == TypeVoid || name == "_" {
		return
	}
	for i := range c.litStack {
		fr := &c.litStack[i]
		if scopeContains(fr.root, declScope) {
			continue // declared inside this lit
		}
		ci := fr.info
		if ci.capSeen == nil {
			ci.capSeen = map[string]bool{}
		}
		if ci.capSeen[name] {
			continue
		}
		ci.capSeen[name] = true
		ci.Captures = append(ci.Captures, CaptureVar{Name: name, Type: t})
		// Promote in the declaring frame.
		promoted := false
		for j := len(c.litStack) - 1; j >= 0; j-- {
			if scopeContains(c.litStack[j].root, declScope) {
				lit := c.litStack[j].lit
				if c.info.PromoteInLit[lit] == nil {
					c.info.PromoteInLit[lit] = map[string]Type{}
				}
				c.info.PromoteInLit[lit][name] = t
				promoted = true
				break
			}
		}
		if !promoted && c.enclosingFn != nil {
			if c.info.PromoteInFunc[c.enclosingFn] == nil {
				c.info.PromoteInFunc[c.enclosingFn] = map[string]Type{}
			}
			c.info.PromoteInFunc[c.enclosingFn][name] = t
		}
	}
}

func (c *Checker) checkFuncLit(e *ast.FuncLit) Type {
	params, variadic := c.paramsFromFields(e.Params)
	for i, p := range e.Params {
		if p.Name == nil {
			c.error(p.Type.Pos(), "function literal parameters must be named")
			if i < len(params) {
				params[i] = TypeInvalid
			}
			continue
		}
		if i < len(params) && (params[i] == TypeInvalid || params[i] == TypeVoid) {
			c.error(p.Type.Pos(), "invalid parameter type")
		}
	}
	results := c.typesFromResults(e.Results)
	for i, r := range results {
		if r == TypeInvalid {
			c.error(e.Results[i].Type.Pos(), "invalid result type")
		}
	}
	ft := c.funcOf(params, results, variadic)

	ci := &ClosureInfo{
		Lit: e, Params: params, Results: results, Variadic: variadic,
		capSeen: map[string]bool{}, ID: c.nextClosure,
	}
	c.nextClosure++
	c.info.Closures[e] = ci

	prevFn := c.curFn
	c.curFn = &funcSig{params: params, results: results, variadic: variadic, decl: nil}
	c.push()
	root := c.scope
	for i, p := range e.Params {
		if p.Name == nil {
			continue
		}
		c.scope.declare(p.Name.Name, params[i], p.Name.Pos(), &c.errs)
	}
	for _, r := range e.Results {
		if r.Name == nil {
			continue
		}
		t := c.typeFromExpr(r.Type)
		c.scope.declare(r.Name.Name, t, r.Name.Pos(), &c.errs)
	}
	c.litStack = append(c.litStack, litFrame{lit: e, root: root, info: ci})
	c.checkBlock(e.Body)
	c.litStack = c.litStack[:len(c.litStack)-1]
	c.pop()
	c.curFn = prevFn
	return ft
}

func (c *Checker) checkBlock(b *ast.BlockStmt) {
	if b == nil {
		return
	}
	c.push()
	for _, s := range b.List {
		c.checkStmt(s)
	}
	c.pop()
}

func (c *Checker) checkStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.VarDecl:
		c.checkVarDecl(s)
	case *ast.ShortVarDecl:
		c.checkShortVar(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.ExprStmt:
		prev := c.discardMulti
		c.discardMulti = true // Go permits discarding any function result(s).
		c.checkExpr(s.X)
		c.discardMulti = prev
	case *ast.IfStmt:
		c.checkIf(s)
	case *ast.SwitchStmt:
		c.checkSwitch(s)
	case *ast.ForStmt:
		c.checkFor(s)
	case *ast.RangeStmt:
		c.checkRange(s)
	case *ast.BreakStmt:
		if c.loopDepth == 0 {
			c.error(s.Pos(), "முறி outside சுழல்")
		}
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.error(s.Pos(), "தொடர் outside சுழல்")
		}
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.DeferStmt:
		c.checkDefer(s)
	case *ast.GoStmt:
		c.checkGo(s)
	case *ast.SendStmt:
		c.checkSend(s)
	case *ast.SelectStmt:
		c.checkSelect(s)
	case *ast.BlockStmt:
		c.checkBlock(s)
	default:
		c.error(s.Pos(), "unsupported statement %T", s)
	}
}

func (c *Checker) checkDefer(s *ast.DeferStmt) {
	if c.curFn == nil {
		c.error(s.Pos(), "தள்ளிவை outside function")
		return
	}
	if s.Call == nil {
		c.error(s.Pos(), "தள்ளிவை requires a function call")
		return
	}
	t := c.checkExpr(s.Call)
	if s.Call.Conversion {
		c.error(s.Call.Pos(), "தள்ளிவை requires a function call, not a conversion")
		return
	}
	_ = t // results of deferred call are discarded (bare name ⇒ zero-arg call)
}

func (c *Checker) checkGo(s *ast.GoStmt) {
	if s.Call == nil {
		c.error(s.Pos(), "இழை requires a function call")
		return
	}
	t := c.checkExpr(s.Call)
	if s.Call.Conversion {
		c.error(s.Call.Pos(), "இழை requires a function call, not a conversion")
		return
	}
	_ = t
}

func (c *Checker) checkSend(s *ast.SendStmt) {
	ct := c.checkExpr(s.Chan)
	vt := c.checkExpr(s.Value)
	if ct == TypeInvalid {
		return
	}
	if !IsChan(ct) {
		c.error(s.Chan.Pos(), "cannot send to non-channel type %s", c.typStr(ct))
		return
	}
	ci := c.info.Chans[ct]
	if ci.Dir == ast.RECV {
		c.error(s.Chan.Pos(), "cannot send on receive-only channel")
		return
	}
	if vt != TypeInvalid && !c.assignable(vt, ci.Elem, s.Value) {
		c.error(s.Value.Pos(), "cannot send %s to channel of %s", c.typStr(vt), c.typStr(ci.Elem))
	}
}

func (c *Checker) checkSelect(s *ast.SelectStmt) {
	hasDefault := false
	for _, cl := range s.Body {
		if cl.Default {
			if hasDefault {
				c.error(cl.Pos(), "multiple மற்றபடி in தடத்தேர்வு")
			}
			hasDefault = true
			c.checkBlock(cl.Body)
			continue
		}
		if cl.Comm == nil {
			c.error(cl.Pos(), "தடத்தேர்வு எனில் requires a send or receive")
			c.checkBlock(cl.Body)
			continue
		}
		c.push()
		c.checkComm(cl.Comm)
		c.checkBlock(cl.Body)
		c.pop()
	}
}

func (c *Checker) checkComm(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.SendStmt:
		c.checkSend(s)
	case *ast.ExprStmt:
		u, ok := s.X.(*ast.UnaryExpr)
		if !ok || u.Op != token.ARROW {
			c.error(s.Pos(), "தடத்தேர்வு எனில் communication must be send or receive")
			c.checkExpr(s.X)
			return
		}
		c.checkExpr(s.X)
	case *ast.AssignStmt:
		if len(s.Values) != 1 {
			c.error(s.Pos(), "தடத்தேர்வு எனில் receive assignment wants one receive")
			return
		}
		u, ok := s.Values[0].(*ast.UnaryExpr)
		if !ok || u.Op != token.ARROW {
			c.error(s.Pos(), "தடத்தேர்வு எனில் assignment value must be a receive")
			c.checkAssign(s)
			return
		}
		c.checkAssign(s)
	case *ast.ShortVarDecl:
		if len(s.Values) != 1 {
			c.error(s.Pos(), "தடத்தேர்வு எனில் receive declaration wants one receive")
			return
		}
		u, ok := s.Values[0].(*ast.UnaryExpr)
		if !ok || u.Op != token.ARROW {
			c.error(s.Pos(), "தடத்தேர்வு எனில் short declaration value must be a receive")
			c.checkShortVar(s)
			return
		}
		c.checkShortVar(s)
	default:
		c.error(s.Pos(), "தடத்தேர்வு எனில் communication must be send or receive")
	}
}

func (c *Checker) checkVarDecl(d *ast.VarDecl) {
	t := c.typeFromExpr(d.Type)
	if t == TypeInvalid {
		c.error(d.Type.Pos(), "invalid type")
		return
	}
	if len(d.Values) != 0 && len(d.Values) != len(d.Names) {
		c.error(d.Pos(), "wrong number of initializers")
	}
	for i, name := range d.Names {
		if i < len(d.Values) {
			vt := c.checkExpr(d.Values[i])
			if !c.assignable(vt, t, d.Values[i]) {
				c.error(d.Values[i].Pos(), "cannot initialize %s as %s", c.typStr(t), c.typStr(vt))
			}
		}
		c.scope.declare(name.Name, t, name.Pos(), &c.errs)
	}
}

func (c *Checker) checkShortVar(d *ast.ShortVarDecl) {
	if len(d.Values) == 1 && len(d.Names) > 1 {
		c.allowMulti = true
		t := c.checkExpr(d.Values[0])
		c.allowMulti = false
		if !IsTuple(t) {
			c.error(d.Pos(), ":= requires matching names and values")
			return
		}
		elems := c.info.TupleElems[t]
		if len(elems) != len(d.Names) {
			c.error(d.Pos(), "assignment mismatch: %d variables but %d values", len(d.Names), len(elems))
			return
		}
		for i, name := range d.Names {
			if name.Name == "_" {
				continue
			}
			if elems[i] == TypeVoid || elems[i] == TypeInvalid {
				c.error(name.Pos(), "cannot infer type for %s", name.Name)
				continue
			}
			c.scope.declare(name.Name, elems[i], name.Pos(), &c.errs)
		}
		return
	}
	if len(d.Values) != len(d.Names) {
		c.error(d.Pos(), ":= requires matching names and values")
		return
	}
	for i, name := range d.Names {
		t := c.checkExpr(d.Values[i])
		if t == TypeVoid || t == TypeInvalid || t == TypeUntypedNil || IsTuple(t) {
			c.error(name.Pos(), "cannot infer type for %s", name.Name)
			continue
		}
		c.scope.declare(name.Name, t, name.Pos(), &c.errs)
	}
}

// assignable reports whether got can be used where want is expected.
// Untyped இன்மை is assignable to any pointer or map type and is re-typed to want.
func (c *Checker) assignable(got, want Type, e ast.Expr) bool {
	if got == TypeInvalid || want == TypeInvalid {
		return true
	}
	if got == TypeUntypedNil {
		if isPointer(want) || IsMap(want) || IsChan(want) {
			c.record(e, want)
			return true
		}
		return false
	}
	if got == want {
		return true
	}
	if IsChan(got) && IsChan(want) {
		g, w := c.info.Chans[got], c.info.Chans[want]
		if g.Elem == w.Elem && (g.Dir == 0 || g.Dir == w.Dir) && !(w.Dir == 0 && g.Dir != 0) {
			return true
		}
		return false
	}
	// Untyped literals may assign to a defined type of matching kind.
	if isDefined(want) {
		if under, ok := c.info.Underlying[want]; ok {
			if lit, ok := e.(*ast.BasicLit); ok {
				if lit.Kind == token.INT && (under == TypeInt || under == TypeByte || under == TypeRune) {
					c.record(e, want)
					return true
				}
				if lit.Kind == token.FLOAT && under == TypeFloat {
					c.record(e, want)
					return true
				}
			}
			if _, ok := e.(*ast.BoolLit); ok && under == TypeBool {
				c.record(e, want)
				return true
			}
		}
	}
	// Untyped int literal → இருமி8 / இருமி32.
	if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.INT && got == TypeInt {
		if want == TypeByte || want == TypeRune {
			c.record(e, want)
			return true
		}
	}
	return false
}

func (c *Checker) checkAssign(s *ast.AssignStmt) {
	if len(s.LHS) == 0 {
		c.error(s.Pos(), "invalid assignment")
		return
	}
	if len(s.LHS) > 1 {
		// Parallel assign: a, b = b, a  (or any matching RHS list).
		if len(s.Values) == len(s.LHS) {
			for i := range s.LHS {
				rt := c.checkExpr(s.Values[i])
				if IsTuple(rt) {
					c.error(s.Values[i].Pos(), "multiple-value in single-value context")
					continue
				}
				c.checkAssignOne(s.LHS[i], rt, s.Values[i])
			}
			return
		}
		// Unpack: a, b = f()
		if len(s.Values) != 1 {
			c.error(s.Pos(), "assignment mismatch: %d variables but %d values", len(s.LHS), len(s.Values))
			return
		}
		c.allowMulti = true
		t := c.checkExpr(s.Values[0])
		c.allowMulti = false
		if !IsTuple(t) {
			c.error(s.Values[0].Pos(), "multi-assign requires a multi-value call or matching RHS list")
			return
		}
		elems := c.info.TupleElems[t]
		if len(elems) != len(s.LHS) {
			c.error(s.Pos(), "assignment mismatch: %d variables but %d values", len(s.LHS), len(elems))
			return
		}
		for i, lhs := range s.LHS {
			c.checkAssignOne(lhs, elems[i], s.Values[0])
		}
		return
	}
	if len(s.Values) != 1 {
		c.error(s.Pos(), "assignment mismatch")
		return
	}
	lhs := s.LHS[0]
	if p, ok := lhs.(*ast.ParenExpr); ok {
		c.checkAssign(&ast.AssignStmt{LHS: []ast.Expr{p.X}, Values: s.Values})
		return
	}
	lt := c.checkAssignTarget(lhs)
	rt := c.checkExpr(s.Values[0])
	if IsTuple(rt) {
		c.error(s.Values[0].Pos(), "multiple-value in single-value context")
		return
	}
	if !c.assignable(rt, lt, s.Values[0]) {
		c.error(s.Values[0].Pos(), "cannot assign %s to %s", c.typStr(rt), c.typStr(lt))
	}
}

func (c *Checker) checkAssignOne(lhs ast.Expr, want Type, src ast.Expr) {
	if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
		return
	}
	lt := c.checkAssignTarget(lhs)
	if !c.assignable(want, lt, src) {
		c.error(lhs.Pos(), "cannot assign %s to %s", c.typStr(want), c.typStr(lt))
	}
}

func (c *Checker) checkAssignTarget(lhs ast.Expr) Type {
	switch lhs := lhs.(type) {
	case *ast.Ident:
		lt, ok := c.scope.lookup(lhs.Name)
		if !ok {
			c.error(lhs.Pos(), "undeclared variable: %s", lhs.Name)
			return TypeInvalid
		}
		return lt
	case *ast.IndexExpr, *ast.SelectorExpr:
		return c.checkExpr(lhs)
	case *ast.UnaryExpr:
		if lhs.Op != token.MUL {
			c.error(lhs.Pos(), "invalid assignment target")
			return TypeInvalid
		}
		return c.checkExpr(lhs)
	case *ast.ParenExpr:
		return c.checkAssignTarget(lhs.X)
	default:
		c.error(lhs.Pos(), "invalid assignment target")
		return TypeInvalid
	}
}

func isAddressable(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name != "_"
	case *ast.IndexExpr, *ast.SelectorExpr:
		return true
	case *ast.UnaryExpr:
		return e.Op == token.MUL
	case *ast.ParenExpr:
		return isAddressable(e.X)
	default:
		return false
	}
}

func (c *Checker) checkIf(s *ast.IfStmt) {
	c.push()
	if s.Init != nil {
		c.checkStmt(s.Init)
	}
	ct := c.checkExpr(s.Cond)
	if ct != TypeBool && ct != TypeInvalid {
		c.error(s.Cond.Pos(), "if condition must be நிலை, got %s", ct)
	}
	c.checkBlock(s.Body)
	if s.Else != nil {
		c.checkStmt(s.Else)
	}
	c.pop()
}

func (c *Checker) checkSwitch(s *ast.SwitchStmt) {
	c.push()
	if s.Init != nil {
		c.checkStmt(s.Init)
	}
	var tagType Type
	if s.Tag != nil {
		tagType = c.checkExpr(s.Tag)
		if tagType != TypeInvalid && tagType != TypeInt && tagType != TypeString && tagType != TypeBool {
			c.error(s.Tag.Pos(), "திசைவி tag must be முழுஎண், சரம், or நிலை, got %s", c.typStr(tagType))
			tagType = TypeInvalid
		}
	}
	sawDefault := false
	for _, cl := range s.Cases {
		if cl.Default {
			if sawDefault {
				c.error(cl.Pos(), "multiple மற்றபடி in திசைவி")
			}
			sawDefault = true
			if len(cl.List) != 0 {
				c.error(cl.Pos(), "மற்றபடி cannot have a value list")
			}
		} else {
			if len(cl.List) == 0 {
				c.error(cl.Pos(), "எனில் case needs at least one expression")
			}
			for _, x := range cl.List {
				xt := c.checkExpr(x)
				if s.Tag == nil {
					if xt != TypeBool && xt != TypeInvalid {
						c.error(x.Pos(), "tagless திசைவி case must be நிலை, got %s", c.typStr(xt))
					}
				} else if xt != TypeInvalid && tagType != TypeInvalid && xt != tagType {
					c.error(x.Pos(), "case type want %s, got %s", c.typStr(tagType), c.typStr(xt))
				}
			}
		}
		c.checkBlock(cl.Body)
	}
	c.pop()
}

func (c *Checker) checkFor(s *ast.ForStmt) {
	c.push()
	if s.Init != nil {
		c.checkStmt(s.Init)
	}
	if s.Cond != nil {
		ct := c.checkExpr(s.Cond)
		if ct != TypeBool && ct != TypeInvalid {
			c.error(s.Cond.Pos(), "சுழல் condition must be நிலை, got %s", ct)
		}
	}
	if s.Post != nil {
		c.checkStmt(s.Post)
	}
	c.loopDepth++
	c.checkBlock(s.Body)
	c.loopDepth--
	c.pop()
}

func isBlank(id *ast.Ident) bool {
	return id != nil && id.Name == "_"
}

func (c *Checker) checkRange(s *ast.RangeStmt) {
	xt := c.checkExpr(s.X)
	var keyT, elem Type
	chanRange := false
	switch {
	case isSlice(xt):
		keyT = TypeInt
		elem = ElemOfSlice(c.info, xt)
	case IsArray(xt):
		keyT = TypeInt
		elem = c.info.Arrays[xt].Elem
	case xt == TypeString:
		keyT = TypeInt
		elem = TypeInt // rune as முழுஎண்
	case IsMap(xt):
		mi := c.info.Maps[xt]
		keyT = mi.Key
		elem = mi.Elem
	case IsChan(xt):
		ci := c.info.Chans[xt]
		if ci.Dir == ast.SEND {
			c.error(s.X.Pos(), "cannot ஒவ்வொரு over send-only தடம்")
		}
		if s.Value != nil {
			c.error(s.Pos(), "ஒவ்வொரு over தடம் allows at most one variable")
		}
		chanRange = true
		keyT = ci.Elem // single ident is the element (Go: for v := range ch)
		elem = TypeInvalid
	case xt == TypeInvalid:
		keyT = TypeInvalid
		elem = TypeInvalid
	default:
		c.error(s.X.Pos(), "ஒவ்வொரு requires a slice, array, சரம், அகராதி, or தடம், got %s", c.typStr(xt))
		xt = TypeInvalid
		keyT = TypeInvalid
		elem = TypeInvalid
	}

	c.push()
	if s.Define {
		if s.Key != nil && !isBlank(s.Key) {
			c.scope.declare(s.Key.Name, keyT, s.Key.Pos(), &c.errs)
		}
		if s.Value != nil && !isBlank(s.Value) && !chanRange {
			if elem == TypeInvalid {
				c.scope.declare(s.Value.Name, TypeInvalid, s.Value.Pos(), &c.errs)
			} else {
				c.scope.declare(s.Value.Name, elem, s.Value.Pos(), &c.errs)
			}
		}
	} else {
		if s.Key != nil && !isBlank(s.Key) {
			kt, ok := c.scope.lookup(s.Key.Name)
			if !ok {
				c.error(s.Key.Pos(), "undeclared variable: %s", s.Key.Name)
			} else if keyT != TypeInvalid && kt != TypeInvalid && kt != keyT {
				c.error(s.Key.Pos(), "range key variable type %s does not match %s", kt, c.typStr(keyT))
			}
		}
		if s.Value != nil && !isBlank(s.Value) && !chanRange {
			vt, ok := c.scope.lookup(s.Value.Name)
			if !ok {
				c.error(s.Value.Pos(), "undeclared variable: %s", s.Value.Name)
			} else if elem != TypeInvalid && vt != TypeInvalid && vt != elem {
				c.error(s.Value.Pos(), "range value variable type %s does not match element %s", vt, c.typStr(elem))
			}
		}
	}
	c.loopDepth++
	c.checkBlock(s.Body)
	c.loopDepth--
	c.pop()
}

func namedResults(rs []*ast.Field) bool {
	if len(rs) == 0 {
		return false
	}
	for _, r := range rs {
		if r == nil || r.Name == nil {
			return false
		}
	}
	return true
}

func (c *Checker) checkReturn(s *ast.ReturnStmt) {
	if c.curFn == nil {
		c.error(s.Pos(), "return outside function")
		return
	}
	want := c.curFn.results
	if len(s.Results) == 0 {
		if len(want) == 0 {
			return
		}
		// Naked return: allowed only when every result is named.
		if c.curFn.decl == nil || !namedResults(c.curFn.decl.Results) {
			c.error(s.Pos(), "missing return values")
		}
		return
	}
	if len(want) == 0 {
		c.error(s.Results[0].Pos(), "unexpected return value")
		return
	}
	if len(s.Results) == 1 && len(want) > 1 {
		c.allowMulti = true
		got := c.checkExpr(s.Results[0])
		c.allowMulti = false
		if IsTuple(got) {
			elems := c.info.TupleElems[got]
			if len(elems) != len(want) {
				c.error(s.Pos(), "wrong number of return values: got %d, want %d", len(elems), len(want))
				return
			}
			for i := range want {
				if !c.assignable(elems[i], want[i], s.Results[0]) {
					c.error(s.Results[0].Pos(), "cannot return %s as %s", c.typStr(elems[i]), c.typStr(want[i]))
				}
			}
			return
		}
	}
	if len(s.Results) != len(want) {
		c.error(s.Pos(), "wrong number of return values: got %d, want %d", len(s.Results), len(want))
		return
	}
	for i, r := range s.Results {
		got := c.checkExpr(r)
		if IsTuple(got) {
			c.error(r.Pos(), "multiple-value in single-value context")
			continue
		}
		if !c.assignable(got, want[i], r) {
			c.error(r.Pos(), "cannot return %s as %s", c.typStr(got), c.typStr(want[i]))
		}
	}
}

func (c *Checker) checkExpr(e ast.Expr) Type {
	var t Type
	switch e := e.(type) {
	case *ast.Ident:
		var ok bool
		var declScope *scope
		t, declScope, ok = c.scope.lookupScope(e.Name)
		if ok {
			if len(c.litStack) > 0 {
				c.noteCapture(e.Name, t, declScope)
			}
		} else {
			if c.cur != nil {
				if sig := c.cur.funcs[e.Name]; sig != nil {
					if sig.generic() {
						c.error(e.Pos(), "cannot use generic function %s as a value", e.Name)
						t = TypeInvalid
					} else {
						t = c.funcOf(sig.params, sig.results, sig.variadic)
						c.info.PkgFuncValues[e] = &PkgFuncValueInfo{
							Pkg: c.cur.name, Name: e.Name,
							Params:  append([]Type(nil), sig.params...),
							Results: append([]Type(nil), sig.results...),
						}
					}
					break
				}
			}
			c.error(e.Pos(), "undeclared: %s", e.Name)
			t = TypeInvalid
		}
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			t = TypeInt
		case token.FLOAT:
			t = TypeFloat
		case token.STRING:
			t = TypeString
		default:
			t = TypeInvalid
		}
	case *ast.BoolLit:
		t = TypeBool
	case *ast.NilLit:
		t = TypeUntypedNil
	case *ast.ParenExpr:
		t = c.checkExpr(e.X)
	case *ast.UnaryExpr:
		xt := c.checkExpr(e.X)
		switch e.Op {
		case token.SUB:
			if xt == TypeFloat {
				t = TypeFloat
			} else {
				if xt != TypeInt && xt != TypeInvalid {
					c.error(e.Pos(), "unary - requires முழுஎண் or மிதவைஎண்")
				}
				t = TypeInt
			}
		case token.NOT:
			if xt != TypeBool && xt != TypeInvalid {
				c.error(e.Pos(), "unary ! requires நிலை")
			}
			t = TypeBool
		case token.MUL:
			if xt == TypeInvalid {
				t = TypeInvalid
			} else if !isPointer(xt) {
				c.error(e.Pos(), "cannot dereference non-pointer %s", c.typStr(xt))
				t = TypeInvalid
			} else {
				t = c.elemOfPtr(xt)
			}
		case token.AND:
			if xt == TypeInvalid {
				t = TypeInvalid
			} else if !isAddressable(e.X) {
				c.error(e.Pos(), "cannot take address of %T", e.X)
				t = TypeInvalid
			} else {
				t = c.pointerOf(xt)
			}
		case token.ARROW:
			if xt == TypeInvalid {
				t = TypeInvalid
			} else if !IsChan(xt) {
				c.error(e.Pos(), "cannot receive from non-channel type %s", c.typStr(xt))
				t = TypeInvalid
			} else {
				ci := c.info.Chans[xt]
				if ci.Dir == ast.SEND {
					c.error(e.Pos(), "cannot receive from send-only channel")
					t = TypeInvalid
				} else if c.allowMulti {
					t = c.tupleOf([]Type{ci.Elem, TypeBool})
				} else {
					t = ci.Elem
				}
			}
		default:
			t = TypeInvalid
		}
	case *ast.BinaryExpr:
		t = c.checkBinary(e)
	case *ast.CallExpr:
		t = c.checkCall(e)
	case *ast.CompositeLit:
		t = c.checkCompositeLit(e)
	case *ast.IndexExpr:
		t = c.checkIndexExpr(e)
	case *ast.SliceExpr:
		t = c.checkSliceExpr(e)
	case *ast.SelectorExpr:
		t = c.checkSelectorExpr(e)
	case *ast.FuncLit:
		t = c.checkFuncLit(e)
	case *ast.KeyValueExpr:
		c.error(e.Pos(), "keyed element outside composite literal")
		t = TypeInvalid
	default:
		c.error(e.Pos(), "unsupported expression %T", e)
		t = TypeInvalid
	}
	return c.record(e, t)
}

func (c *Checker) checkCompositeLit(e *ast.CompositeLit) Type {
	t := c.typeFromExpr(e.Type)
	if IsMap(t) {
		mi := c.info.Maps[t]
		for _, el := range e.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				c.error(el.Pos(), "missing key in map literal")
				c.checkExpr(el)
				continue
			}
			kt := c.checkExpr(kv.Key)
			vt := c.checkExpr(kv.Value)
			if kt != TypeInvalid && !c.assignable(kt, mi.Key, kv.Key) {
				c.error(kv.Key.Pos(), "map key want %s, got %s", c.typStr(mi.Key), c.typStr(kt))
			}
			if vt != TypeInvalid && !c.assignable(vt, mi.Elem, kv.Value) {
				c.error(kv.Value.Pos(), "map value want %s, got %s", c.typStr(mi.Elem), c.typStr(vt))
			}
		}
		return t
	}
	if isSlice(t) || IsArray(t) {
		var elem Type
		if isSlice(t) {
			elem = ElemOfSlice(c.info, t)
		} else {
			elem = c.info.Arrays[t].Elem
			if int64(len(e.Elts)) > c.info.Arrays[t].Len {
				c.error(e.Pos(), "too many elements in array literal")
			}
		}
		for _, el := range e.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				c.error(kv.Pos(), "keyed elements not allowed in slice/array literal")
				c.checkExpr(kv.Value)
				continue
			}
			et := c.checkExpr(el)
			if et != TypeInvalid && elem != TypeInvalid && !c.assignable(et, elem, el) {
				c.error(el.Pos(), "element type want %s, got %s", c.typStr(elem), c.typStr(et))
			}
		}
		return t
	}
	if isStruct(t) {
		si := c.info.Structs[t]
		foreign := c.cur != nil && si.Pkg != c.cur.name
		if len(e.Elts) == 0 {
			return t
		}
		_, keyed := e.Elts[0].(*ast.KeyValueExpr)
		if keyed {
			fieldType := map[string]Type{}
			fieldExp := map[string]bool{}
			for _, f := range si.Fields {
				fieldType[f.Name] = f.Type
				fieldExp[f.Name] = f.Exported
			}
			seen := map[string]bool{}
			for _, el := range e.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					c.error(el.Pos(), "cannot mix keyed and unkeyed fields in struct literal")
					c.checkExpr(el)
					continue
				}
				keyIdent, ok := kv.Key.(*ast.Ident)
				if !ok {
					c.error(kv.Key.Pos(), "invalid field name in struct literal")
					c.checkExpr(kv.Value)
					continue
				}
				ft, ok := fieldType[keyIdent.Name]
				if !ok {
					c.error(keyIdent.Pos(), "unknown field %s", keyIdent.Name)
					c.checkExpr(kv.Value)
					continue
				}
				if foreign && !fieldExp[keyIdent.Name] {
					c.error(keyIdent.Pos(), "field %s.%s is not exported (need வெளி)", si.Name, keyIdent.Name)
					c.checkExpr(kv.Value)
					continue
				}
				if seen[keyIdent.Name] {
					c.error(keyIdent.Pos(), "duplicate field %s in literal", keyIdent.Name)
				}
				seen[keyIdent.Name] = true
				vt := c.checkExpr(kv.Value)
				if !c.assignable(vt, ft, kv.Value) {
					c.error(kv.Value.Pos(), "field %s want %s, got %s", keyIdent.Name, c.typStr(ft), c.typStr(vt))
				}
			}
			return t
		}
		// Positional (unkeyed): declaration order, exact count (Go).
		if foreign {
			for _, f := range si.Fields {
				if !f.Exported {
					c.error(e.Pos(), "cannot use unkeyed literal for %s: field %s is not exported", si.Name, f.Name)
					break
				}
			}
		}
		if len(e.Elts) != len(si.Fields) {
			c.error(e.Pos(), "wrong number of values in struct literal: got %d, want %d", len(e.Elts), len(si.Fields))
		}
		n := len(e.Elts)
		if n > len(si.Fields) {
			n = len(si.Fields)
		}
		for i := 0; i < n; i++ {
			el := e.Elts[i]
			if _, ok := el.(*ast.KeyValueExpr); ok {
				c.error(el.Pos(), "cannot mix keyed and unkeyed fields in struct literal")
				continue
			}
			vt := c.checkExpr(el)
			ft := si.Fields[i].Type
			if !c.assignable(vt, ft, el) {
				c.error(el.Pos(), "field %s want %s, got %s", si.Fields[i].Name, c.typStr(ft), c.typStr(vt))
			}
		}
		for i := n; i < len(e.Elts); i++ {
			el := e.Elts[i]
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				c.error(el.Pos(), "cannot mix keyed and unkeyed fields in struct literal")
				c.checkExpr(kv.Value)
				continue
			}
			c.checkExpr(el)
		}
		return t
	}
	c.error(e.Pos(), "composite literal requires slice, array, struct, or அகராதி type")
	for _, el := range e.Elts {
		c.checkExpr(el)
	}
	return TypeInvalid
}

func (c *Checker) checkSelectorExpr(e *ast.SelectorExpr) Type {
	if id, ok := e.X.(*ast.Ident); ok && c.cur != nil {
		if imp, ok := c.cur.imports[id.Name]; ok {
			c.markImportUsed(id.Name)
			if sig := imp.funcs[e.Sel.Name]; sig != nil {
				if !sig.exported {
					c.error(e.Sel.Pos(), "function %s.%s is not exported (need வெளி)", imp.name, e.Sel.Name)
					return TypeInvalid
				}
				if sig.generic() {
					c.error(e.Pos(), "cannot use generic function %s.%s as a value", imp.name, e.Sel.Name)
					return TypeInvalid
				}
				ft := c.funcOf(sig.params, sig.results, sig.variadic)
				c.info.PkgFuncValues[e] = &PkgFuncValueInfo{
					Pkg: imp.name, Name: e.Sel.Name,
					Params:  append([]Type(nil), sig.params...),
					Results: append([]Type(nil), sig.results...),
				}
				return ft
			}
			c.error(e.Sel.Pos(), "package %s has no function %s", id.Name, e.Sel.Name)
			return TypeInvalid
		}
	}
	// Tamil-0.47: method expression T.M or (*T).M
	if ft, ok := c.tryMethodExpr(e); ok {
		return ft
	}
	xt := c.checkExpr(e.X)
	if xt == TypeInvalid {
		return TypeInvalid
	}
	recvIsPtr := isPointer(xt)
	base := xt
	if recvIsPtr {
		base = c.elemOfPtr(xt)
	}
	if !isStruct(base) {
		c.error(e.Pos(), "cannot select field from %s", c.typStr(c.info.Types[e.X]))
		return TypeInvalid
	}
	si := c.info.Structs[base]
	for _, f := range si.Fields {
		if f.Name == e.Sel.Name {
			if c.cur != nil && si.Pkg != c.cur.name && !f.Exported {
				c.error(e.Sel.Pos(), "field %s.%s is not exported (need வெளி)", si.Name, f.Name)
				return TypeInvalid
			}
			return f.Type
		}
	}
	if mi, ok := si.Methods[e.Sel.Name]; ok {
		if c.cur != nil && si.Pkg != c.cur.name && !mi.Exported {
			c.error(e.Sel.Pos(), "method %s.%s is not exported (need வெளி)", si.Name, mi.Name)
			return TypeInvalid
		}
		takeAddr := false
		if mi.RecvIsPtr {
			if !recvIsPtr && !isAddressable(e.X) {
				c.error(e.X.Pos(), "cannot take method value %s with pointer receiver on non-addressable value", mi.Name)
				return TypeInvalid
			}
			if !recvIsPtr {
				takeAddr = true
			}
		}
		ft := c.funcOf(mi.Params, mi.Results, mi.Variadic)
		c.info.MethodValues[e] = &MethodValueInfo{
			Method: mi, Struct: si, TakeAddr: takeAddr, RecvIsPtr: recvIsPtr,
		}
		return ft
	}
	c.error(e.Sel.Pos(), "type %s has no field or method %s", si.Name, e.Sel.Name)
	return TypeInvalid
}

// tryMethodExpr handles T.M / (*T).M. ok is false if X is not a type expression.
func (c *Checker) tryMethodExpr(e *ast.SelectorExpr) (Type, bool) {
	base, exprPtr, ok := c.resolveMethodExprRecv(e.X)
	if !ok {
		return TypeInvalid, false
	}
	if !isStruct(base) {
		c.error(e.X.Pos(), "method expression requires a named struct type")
		return TypeInvalid, true
	}
	si := c.info.Structs[base]
	mi, found := si.Methods[e.Sel.Name]
	if !found {
		c.error(e.Sel.Pos(), "type %s has no method %s", si.Name, e.Sel.Name)
		return TypeInvalid, true
	}
	if c.cur != nil && si.Pkg != c.cur.name && !mi.Exported {
		c.error(e.Sel.Pos(), "method %s.%s is not exported (need வெளி)", si.Name, mi.Name)
		return TypeInvalid, true
	}
	// Method set of T: value receivers only. Method set of *T: both.
	if !exprPtr && mi.RecvIsPtr {
		c.error(e.Sel.Pos(), "method %s has pointer receiver; use (*%s).%s", mi.Name, si.Name, mi.Name)
		return TypeInvalid, true
	}
	recvType := base
	if exprPtr {
		recvType = c.pointerOf(base)
	}
	params := append([]Type{recvType}, mi.Params...)
	ft := c.funcOf(params, mi.Results, mi.Variadic)
	c.info.MethodExprs[e] = &MethodExprInfo{
		Method: mi, Struct: si, ExprRecvPtr: exprPtr, RecvType: recvType,
	}
	return ft, true
}

// resolveMethodExprRecv returns the named struct type and whether the expression is (*T).
// ok is false when X is not a type form (fall through to value selector).
func (c *Checker) resolveMethodExprRecv(x ast.Expr) (base Type, exprPtr bool, ok bool) {
	switch x := x.(type) {
	case *ast.Ident:
		// Variable shadows type name.
		if _, exists := c.scope.lookup(x.Name); exists {
			return TypeInvalid, false, false
		}
		if c.cur == nil {
			return TypeInvalid, false, false
		}
		t, found := c.cur.types[x.Name]
		if !found || t == TypeInvalid {
			return TypeInvalid, false, false
		}
		if isStruct(t) {
			return t, false, true
		}
		// Defined types are not method receivers in Tamil-0.8+.
		return TypeInvalid, false, false
	case *ast.ParenExpr:
		u, ok := x.X.(*ast.UnaryExpr)
		if !ok || u.Op != token.MUL {
			return TypeInvalid, false, false
		}
		id, ok := u.X.(*ast.Ident)
		if !ok {
			return TypeInvalid, false, false
		}
		if _, exists := c.scope.lookup(id.Name); exists {
			return TypeInvalid, false, false
		}
		if c.cur == nil {
			return TypeInvalid, false, false
		}
		t, found := c.cur.types[id.Name]
		if !found || t == TypeInvalid || !isStruct(t) {
			return TypeInvalid, false, false
		}
		return t, true, true
	default:
		return TypeInvalid, false, false
	}
}

func (c *Checker) checkIndexExpr(e *ast.IndexExpr) Type {
	xt := c.checkExpr(e.X)
	it := c.checkExpr(e.Index)
	if xt == TypeInvalid {
		return TypeInvalid
	}
	if IsMap(xt) {
		mi := c.info.Maps[xt]
		if it != TypeInvalid && !c.assignable(it, mi.Key, e.Index) {
			c.error(e.Index.Pos(), "map key want %s, got %s", c.typStr(mi.Key), c.typStr(it))
		}
		if c.allowMulti {
			return c.tupleOf([]Type{mi.Elem, TypeBool})
		}
		return mi.Elem
	}
	if it != TypeInt && it != TypeInvalid {
		c.error(e.Index.Pos(), "index must be முழுஎண்")
	}
	if isSlice(xt) {
		return ElemOfSlice(c.info, xt)
	}
	if IsArray(xt) {
		return c.info.Arrays[xt].Elem
	}
	c.error(e.Pos(), "cannot index %s", c.typStr(xt))
	return TypeInvalid
}

func (c *Checker) checkSliceExpr(e *ast.SliceExpr) Type {
	xt := c.checkExpr(e.X)
	if e.Low != nil {
		lt := c.checkExpr(e.Low)
		if lt != TypeInt && lt != TypeInvalid {
			c.error(e.Low.Pos(), "slice low bound must be முழுஎண்")
		}
	}
	if e.High != nil {
		ht := c.checkExpr(e.High)
		if ht != TypeInt && ht != TypeInvalid {
			c.error(e.High.Pos(), "slice high bound must be முழுஎண்")
		}
	}
	if e.Max != nil || e.Slice3 {
		if e.High == nil {
			c.error(e.Pos(), "middle index required in three-index slice")
		}
		if e.Max != nil {
			mt := c.checkExpr(e.Max)
			if mt != TypeInt && mt != TypeInvalid {
				c.error(e.Max.Pos(), "slice max bound must be முழுஎண்")
			}
		}
		if xt == TypeString {
			c.error(e.Pos(), "three-index slice not allowed on சரம்")
			return TypeInvalid
		}
	}
	if xt == TypeInvalid {
		return TypeInvalid
	}
	if isSlice(xt) || xt == TypeString {
		return xt
	}
	if IsArray(xt) {
		ai := c.info.Arrays[xt]
		st, ok := c.sliceOf(ai.Elem)
		if !ok {
			c.error(e.Pos(), "cannot slice array of %s", c.typStr(ai.Elem))
			return TypeInvalid
		}
		return st
	}
	c.error(e.Pos(), "cannot slice %s", c.typStr(xt))
	return TypeInvalid
}

func (c *Checker) checkBinary(e *ast.BinaryExpr) Type {
	lt := c.checkExpr(e.X)
	rt := c.checkExpr(e.Y)
	switch e.Op {
	case token.ADD:
		if lt == TypeString || rt == TypeString {
			if lt != TypeInvalid && rt != TypeInvalid && (lt != TypeString || rt != TypeString) {
				c.error(e.OpPos, "string concatenation requires சரம் operands")
			}
			return TypeString
		}
		if c.isNumeric(lt) && c.isNumeric(rt) {
			if lt == TypeFloat || rt == TypeFloat {
				return TypeFloat
			}
			if lt == rt {
				return lt
			}
			return TypeInt
		}
		if lt != TypeInvalid && rt != TypeInvalid {
			c.error(e.OpPos, "arithmetic requires numeric operands")
		}
		return TypeInt
	case token.SUB, token.MUL, token.QUO:
		if c.isNumeric(lt) && c.isNumeric(rt) {
			if lt == TypeFloat || rt == TypeFloat {
				return TypeFloat
			}
			if lt == rt {
				return lt
			}
			return TypeInt
		}
		if lt != TypeInvalid && rt != TypeInvalid {
			c.error(e.OpPos, "arithmetic requires numeric operands")
		}
		return TypeInt
	case token.REM:
		if (!c.isInteger(lt) && lt != TypeInvalid) || (!c.isInteger(rt) && rt != TypeInvalid) {
			c.error(e.OpPos, "%% requires integer operands")
		}
		if lt == rt && c.isInteger(lt) {
			return lt
		}
		return TypeInt
	case token.EQL, token.NEQ:
		if lt == TypeUntypedNil && rt == TypeUntypedNil {
			return TypeBool
		}
		if lt == TypeUntypedNil && (isPointer(rt) || IsMap(rt) || IsChan(rt)) {
			c.record(e.X, rt)
			return TypeBool
		}
		if rt == TypeUntypedNil && (isPointer(lt) || IsMap(lt) || IsChan(lt)) {
			c.record(e.Y, lt)
			return TypeBool
		}
		if isPointer(lt) || isPointer(rt) {
			if !isPointer(lt) || !isPointer(rt) || lt != rt {
				c.error(e.OpPos, "mismatched pointer types in comparison")
			}
			return TypeBool
		}
		if IsMap(lt) || IsMap(rt) {
			c.error(e.OpPos, "maps can only be compared to இன்மை")
			return TypeBool
		}
		if lt == TypeUntypedNil || rt == TypeUntypedNil {
			c.error(e.OpPos, "இன்மை can only compare with a pointer or அகராதி")
			return TypeBool
		}
		if lt != TypeInvalid && rt != TypeInvalid && lt != rt {
			c.error(e.OpPos, "mismatched types in comparison")
			return TypeBool
		}
		if lt != TypeInvalid && !c.comparable(lt) {
			c.error(e.OpPos, "comparison not supported for %s", c.typStr(lt))
		}
		return TypeBool
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		if c.isNumeric(lt) && c.isNumeric(rt) {
			return TypeBool
		}
		if lt != TypeInvalid && rt != TypeInvalid {
			c.error(e.OpPos, "ordered comparison requires முழுஎண் or மிதவைஎண்")
		}
		return TypeBool
	default:
		c.error(e.OpPos, "unknown operator")
		return TypeInvalid
	}
}

func (c *Checker) isNumeric(t Type) bool {
	return t == TypeInt || t == TypeFloat || t == TypeByte || t == TypeRune
}

func (c *Checker) isInteger(t Type) bool {
	return t == TypeInt || t == TypeByte || t == TypeRune
}

func (c *Checker) underlying(t Type) Type {
	if isDefined(t) {
		if u, ok := c.info.Underlying[t]; ok {
			return u
		}
	}
	return t
}

// convertible reports whether got may be converted to want (Go-like).
func (c *Checker) convertible(got, want Type) bool {
	if got == TypeInvalid || want == TypeInvalid {
		return true
	}
	if got == want {
		return true
	}
	ug, uw := c.underlying(got), c.underlying(want)
	if ug == uw {
		return true
	}
	// Numeric integers/floats (after peeling defined types).
	if c.isNumeric(ug) && c.isNumeric(uw) {
		return true
	}
	// Pointers: *T ↔ *U when T and U have identical underlying types.
	if isPointer(ug) && isPointer(uw) {
		return c.underlying(c.elemOfPtr(ug)) == c.underlying(c.elemOfPtr(uw))
	}
	// Integer / இருமி32 → சரம் (UTF-8 code point).
	if uw == TypeString && (ug == TypeInt || ug == TypeRune || ug == TypeByte) {
		return true
	}
	// சரம் ↔ []இருமி8 / []இருமி32
	if ug == TypeString && (uw == TypeSliceByte || uw == TypeSliceRune) {
		return true
	}
	if uw == TypeString && (ug == TypeSliceByte || ug == TypeSliceRune) {
		return true
	}
	return false
}

// typeFromConversionFun resolves T, (*T), []T, or (T) used as the Fun of T(x).
func (c *Checker) typeFromConversionFun(fun ast.Expr) (Type, bool) {
	switch fun := fun.(type) {
	case *ast.Ident:
		t := c.lookupTypeName(fun.Name)
		return t, t != TypeInvalid
	case *ast.SliceType:
		t := c.typeFromExpr(fun)
		return t, t != TypeInvalid
	case *ast.ParenExpr:
		return c.typeFromConversionFun(fun.X)
	case *ast.UnaryExpr:
		if fun.Op == token.MUL {
			if elem, ok := c.typeFromConversionFun(fun.X); ok {
				return c.pointerOf(elem), true
			}
		}
	}
	return TypeInvalid, false
}

func (c *Checker) lookupTypeName(name string) Type {
	switch name {
	case "முழுஎண்":
		return TypeInt
	case "நிலை":
		return TypeBool
	case "சரம்":
		return TypeString
	case "மிதவைஎண்":
		return TypeFloat
	case "இருமி8":
		return TypeByte
	case "இருமி32":
		return TypeRune
	}
	if c.typeParamEnv != nil {
		if t, ok := c.typeParamEnv[name]; ok {
			return t
		}
	}
	if c.cur != nil {
		if t, ok := c.cur.types[name]; ok && t != TypeInvalid {
			return t
		}
	}
	return TypeInvalid
}

func (c *Checker) checkConversion(e *ast.CallExpr, dest Type) Type {
	e.Conversion = true
	if len(e.Args) != 1 {
		c.error(e.Pos(), "conversion requires exactly one argument")
		return dest
	}
	got := c.checkExpr(e.Args[0])
	if IsTuple(got) {
		c.error(e.Args[0].Pos(), "multiple-value in single-value context")
		return dest
	}
	if got != TypeInvalid && dest != TypeInvalid && !c.convertible(got, dest) {
		c.error(e.Pos(), "cannot convert %s to %s", c.typStr(got), c.typStr(dest))
	}
	return dest
}

func (c *Checker) checkCall(e *ast.CallExpr) Type {
	if e.Builtin {
		id, _ := e.Fun.(*ast.Ident)
		name := "பதிப்பி"
		if id != nil {
			name = id.Name
		}
		switch name {
		case "சேர்":
			if len(e.Args) < 2 {
				c.error(e.Pos(), "சேர் requires a slice and at least one element")
				return TypeInvalid
			}
			st := c.checkExpr(e.Args[0])
			if st != TypeInvalid && !isSlice(st) {
				c.error(e.Args[0].Pos(), "சேர் first argument must be a slice")
				return TypeInvalid
			}
			want := ElemOfSlice(c.info, st)
			for i := 1; i < len(e.Args); i++ {
				et := c.checkExpr(e.Args[i])
				if !c.assignable(et, want, e.Args[i]) {
					c.error(e.Args[i].Pos(), "சேர் element want %s, got %s", c.typStr(want), c.typStr(et))
				}
			}
			return st
		case "நீளம்":
			if len(e.Args) != 1 {
				c.error(e.Pos(), "நீளம் takes exactly one argument")
				return TypeInt
			}
			t := c.checkExpr(e.Args[0])
			if t != TypeInvalid && !isSlice(t) && !IsArray(t) && !IsMap(t) && t != TypeString {
				c.error(e.Args[0].Pos(), "நீளம் requires a slice, array, அகராதி, or சரம்")
			}
			return TypeInt
		case "நீக்கு":
			if len(e.Args) != 2 {
				c.error(e.Pos(), "நீக்கு takes a map and a key")
				return TypeVoid
			}
			mt := c.checkExpr(e.Args[0])
			kt := c.checkExpr(e.Args[1])
			if mt != TypeInvalid && !IsMap(mt) {
				c.error(e.Args[0].Pos(), "நீக்கு first argument must be an அகராதி")
				return TypeVoid
			}
			if IsMap(mt) {
				want := c.info.Maps[mt].Key
				if kt != TypeInvalid && !c.assignable(kt, want, e.Args[1]) {
					c.error(e.Args[1].Pos(), "நீக்கு key want %s, got %s", c.typStr(want), c.typStr(kt))
				}
			}
			return TypeVoid
		case "மூடு":
			if len(e.Args) != 1 {
				c.error(e.Pos(), "மூடு takes exactly one channel")
				return TypeVoid
			}
			ct := c.checkExpr(e.Args[0])
			if ct != TypeInvalid && !IsChan(ct) {
				c.error(e.Args[0].Pos(), "மூடு argument must be a தடம்")
			} else if IsChan(ct) && c.info.Chans[ct].Dir == ast.RECV {
				c.error(e.Args[0].Pos(), "cannot close receive-only channel")
			}
			return TypeVoid
		case "அலறு":
			if len(e.Args) != 1 {
				c.error(e.Pos(), "அலறு takes exactly one argument")
				return TypeVoid
			}
			t := c.checkExpr(e.Args[0])
			u := c.underlying(t)
			if t != TypeInvalid && !isPanicArgType(u) {
				c.error(e.Args[0].Pos(), "அலறு argument must be சரம், முழுஎண், மிதவைஎண், நிலை, இருமி8, or இருமி32")
			}
			return TypeVoid
		case "மீள்":
			if len(e.Args) != 0 {
				c.error(e.Pos(), "மீள் takes no arguments")
			}
			for _, a := range e.Args {
				c.checkExpr(a)
			}
			return TypeString
		case "திறன்":
			if len(e.Args) != 1 {
				c.error(e.Pos(), "திறன் takes exactly one argument")
				return TypeInt
			}
			t := c.checkExpr(e.Args[0])
			if t != TypeInvalid && !isSlice(t) && !IsArray(t) {
				c.error(e.Args[0].Pos(), "திறன் requires a slice or array")
			}
			return TypeInt
		case "நகல்":
			if len(e.Args) != 2 {
				c.error(e.Pos(), "நகல் takes exactly two arguments")
				return TypeInt
			}
			dt := c.checkExpr(e.Args[0])
			st := c.checkExpr(e.Args[1])
			if dt != TypeInvalid && !isSlice(dt) {
				c.error(e.Args[0].Pos(), "நகல் destination must be a slice")
			}
			if st != TypeInvalid && !isSlice(st) {
				c.error(e.Args[1].Pos(), "நகல் source must be a slice")
			}
			if dt != TypeInvalid && st != TypeInvalid && dt != st {
				c.error(e.Pos(), "நகல் arguments must be the same slice type")
			}
			return TypeInt
		case "ஆக்கு":
			if e.TypeArg == nil {
				c.error(e.Pos(), "ஆக்கு requires a type argument")
				return TypeInvalid
			}
			st := c.typeFromExpr(e.TypeArg)
			if IsMap(st) {
				if len(e.Args) > 1 {
					c.error(e.Pos(), "ஆக்கு(அகராதி) takes an optional size hint only")
					return TypeInvalid
				}
				for _, a := range e.Args {
					at := c.checkExpr(a)
					if at != TypeInvalid && at != TypeInt {
						c.error(a.Pos(), "ஆக்கு map size hint must be முழுஎண்")
					}
				}
				return st
			}
			if IsChan(st) {
				if len(e.Args) > 1 {
					c.error(e.Pos(), "ஆக்கு(தடம்) takes an optional buffer size only")
					return TypeInvalid
				}
				for _, a := range e.Args {
					at := c.checkExpr(a)
					if at != TypeInvalid && at != TypeInt {
						c.error(a.Pos(), "ஆக்கு channel buffer size must be முழுஎண்")
					}
				}
				return st
			}
			if len(e.Args) != 1 && len(e.Args) != 2 {
				c.error(e.Pos(), "ஆக்கு takes type, length, and optional capacity")
				return TypeInvalid
			}
			if st != TypeInvalid && !isSlice(st) {
				c.error(e.TypeArg.Pos(), "ஆக்கு supports slice, அகராதி, or தடம் types")
				st = TypeInvalid
			}
			for _, a := range e.Args {
				at := c.checkExpr(a)
				if at != TypeInvalid && at != TypeInt {
					c.error(a.Pos(), "ஆக்கு length/capacity must be முழுஎண்")
				}
			}
			return st
		default: // பதிப்பி
			if len(e.Args) != 1 {
				c.error(e.Pos(), "%s takes exactly one argument", name)
				return TypeVoid
			}
			t := c.checkExpr(e.Args[0])
			if isStruct(t) {
				// Tamil-0.17: named structs OK
			} else if isDefined(t) {
				// Print via underlying (emit follows Underlying).
			} else if isSlice(t) || isPointer(t) || IsMap(t) || IsFunc(t) || t == TypeUntypedNil {
				c.error(e.Args[0].Pos(), "பதிப்பி cannot print a slice, pointer, அகராதி, செயல்பாடு, or இன்மை")
			} else if t != TypeInt && t != TypeFloat && t != TypeString && t != TypeBool && t != TypeInvalid {
				c.error(e.Args[0].Pos(), "பதிப்பி expects முழுஎண், மிதவைஎண், சரம், நிலை, or a struct")
			}
			return TypeVoid
		}
	}
	explicit := c.explicitTypeArgs(e)

	if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && c.cur != nil {
			if imp, ok := c.cur.imports[id.Name]; ok {
				c.markImportUsed(id.Name)
				if t, ok := imp.types[sel.Sel.Name]; ok {
					if _, isFunc := imp.funcs[sel.Sel.Name]; isFunc {
						c.error(sel.Sel.Pos(), "%s.%s is both a function and a type", id.Name, sel.Sel.Name)
						return TypeInvalid
					}
					if !imp.typeExp[sel.Sel.Name] {
						c.error(sel.Sel.Pos(), "type %s.%s is not exported (need வெளி)", id.Name, sel.Sel.Name)
						return TypeInvalid
					}
					return c.checkConversion(e, t)
				}
				return c.checkPkgFuncCall(e, sel, imp, explicit)
			}
		}
		return c.checkSelectorCall(e, sel)
	}
	if dest, ok := c.typeFromConversionFun(e.Fun); ok {
		if id, ok := e.Fun.(*ast.Ident); ok && c.cur != nil {
			if _, isFunc := c.cur.funcs[id.Name]; isFunc {
				c.error(id.Pos(), "%s is both a function and a type", id.Name)
				return TypeInvalid
			}
		}
		return c.checkConversion(e, dest)
	}
	funName := ""
	var funPos token.Pos
	pkgName := ""
	var sig *funcSig
	switch fun := e.Fun.(type) {
	case *ast.Ident:
		if t, ok := c.scope.lookup(fun.Name); ok {
			if IsFunc(t) {
				return c.checkFuncValueCall(e, t)
			}
			c.error(fun.Pos(), "call of non-function %s", fun.Name)
			for _, arg := range e.Args {
				c.checkExpr(arg)
			}
			return TypeInvalid
		}
		funName = fun.Name
		funPos = fun.Pos()
		if c.cur != nil {
			sig = c.cur.funcs[funName]
			pkgName = c.cur.name
		}
	case *ast.IndexExpr:
		switch x := fun.X.(type) {
		case *ast.Ident:
			funName = x.Name
			funPos = x.Pos()
			if c.cur != nil {
				sig = c.cur.funcs[funName]
				pkgName = c.cur.name
			}
			if len(explicit) == 0 {
				et := c.typeArgFromExpr(fun.Index)
				if et == TypeInvalid {
					c.error(fun.Index.Pos(), "invalid type argument")
				} else {
					explicit = []Type{et}
				}
			}
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if !ok || c.cur == nil {
				c.error(e.Pos(), "call of non-function")
				return TypeInvalid
			}
			imp, ok := c.cur.imports[id.Name]
			if !ok {
				c.error(id.Pos(), "undefined package: %s", id.Name)
				return TypeInvalid
			}
			c.markImportUsed(id.Name)
			funName = x.Sel.Name
			funPos = x.Sel.Pos()
			sig = imp.funcs[funName]
			pkgName = imp.name
			if sig != nil && !sig.exported {
				c.error(funPos, "function %s.%s is not exported (need வெளி)", imp.name, funName)
				return TypeInvalid
			}
			if len(explicit) == 0 {
				et := c.typeArgFromExpr(fun.Index)
				if et == TypeInvalid {
					c.error(fun.Index.Pos(), "invalid type argument")
				} else {
					explicit = []Type{et}
				}
			}
		default:
			c.error(e.Pos(), "call of non-function")
			return TypeInvalid
		}
	default:
		ft := c.checkExpr(e.Fun)
		if IsFunc(ft) {
			return c.checkFuncValueCall(e, ft)
		}
		c.error(e.Pos(), "call of non-function")
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	if sig == nil {
		c.error(funPos, "undefined function: %s", funName)
		return TypeInvalid
	}
	if sig.generic() {
		return c.checkGenericCall(e, sig, pkgName, funName, explicit)
	}
	if len(explicit) > 0 {
		c.error(e.Pos(), "cannot instantiate non-generic function %s", funName)
	}
	c.checkCallArgs(e, sig.params, sig.variadic, funName)
	return c.finishCallResult(e, sig.results)
}

func (c *Checker) explicitTypeArgs(e *ast.CallExpr) []Type {
	if e == nil || len(e.TypeArgs) == 0 {
		return nil
	}
	out := make([]Type, len(e.TypeArgs))
	for i, te := range e.TypeArgs {
		out[i] = c.typeFromExpr(te)
		if out[i] == TypeInvalid {
			c.error(te.Pos(), "invalid type argument")
		}
	}
	return out
}

func (c *Checker) finishCallResult(e *ast.CallExpr, results []Type) Type {
	t := c.typeFromResultList(results)
	if IsTuple(t) && !c.allowMulti && !c.discardMulti {
		c.error(e.Pos(), "multiple-value in single-value context")
		return TypeInvalid
	}
	return t
}

func (c *Checker) checkPkgFuncCall(e *ast.CallExpr, sel *ast.SelectorExpr, imp *pkgState, explicit []Type) Type {
	sig, ok := imp.funcs[sel.Sel.Name]
	if !ok {
		c.error(sel.Sel.Pos(), "package %s has no function %s", imp.name, sel.Sel.Name)
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	if !sig.exported {
		c.error(sel.Sel.Pos(), "function %s.%s is not exported (need வெளி)", imp.name, sel.Sel.Name)
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	if sig.generic() {
		return c.checkGenericCall(e, sig, imp.name, sel.Sel.Name, explicit)
	}
	if len(explicit) > 0 {
		c.error(e.Pos(), "cannot instantiate non-generic function %s.%s", imp.name, sel.Sel.Name)
	}
	c.checkCallArgs(e, sig.params, sig.variadic, imp.name+"."+sel.Sel.Name)
	return c.finishCallResult(e, sig.results)
}

func (c *Checker) typeArgFromExpr(e ast.Expr) Type {
	switch e := e.(type) {
	case *ast.Ident:
		return c.lookupTypeName(e.Name)
	case *ast.ParenExpr:
		return c.typeArgFromExpr(e.X)
	default:
		// TYPE_* tokens become Idents in parseOperand; other forms unsupported in MVP.
		return TypeInvalid
	}
}

func (c *Checker) checkGenericCall(e *ast.CallExpr, sig *funcSig, pkg, name string, explicit []Type) Type {
	argTypes := make([]Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = c.checkExpr(arg)
	}
	if sig.variadic {
		if len(sig.params) == 0 {
			c.error(e.Pos(), "invalid variadic signature for %s", name)
			return TypeInvalid
		}
		fixed := len(sig.params) - 1
		if len(e.Args) < fixed {
			c.error(e.Pos(), "wrong number of arguments to %s (want at least %d, got %d)", name, fixed, len(e.Args))
		}
	} else if len(e.Args) != len(sig.params) {
		c.error(e.Pos(), "wrong number of arguments to %s (want %d, got %d)", name, len(sig.params), len(e.Args))
	}
	subst := map[Type]Type{}
	if len(explicit) > 0 {
		if len(explicit) != len(sig.typeParams) {
			c.error(e.Pos(), "wrong number of type arguments to %s (want %d, got %d)", name, len(sig.typeParams), len(explicit))
		} else {
			for i, tp := range sig.typeParams {
				if i < len(explicit) && explicit[i] != TypeInvalid {
					subst[tp] = explicit[i]
				}
			}
		}
	}
	if sig.variadic && len(sig.params) > 0 {
		fixed := len(sig.params) - 1
		for i := 0; i < fixed && i < len(argTypes); i++ {
			if argTypes[i] == TypeInvalid {
				continue
			}
			if !c.unify(sig.params[i], argTypes[i], subst) {
				c.error(e.Args[i].Pos(), "argument %d: cannot infer type parameter (want %s, got %s)",
					i+1, c.typStr(sig.params[i]), c.typStr(argTypes[i]))
			}
		}
		elemSch := ElemOfSlice(c.info, sig.params[fixed])
		for i := fixed; i < len(argTypes); i++ {
			if argTypes[i] == TypeInvalid {
				continue
			}
			if !c.unify(elemSch, argTypes[i], subst) {
				c.error(e.Args[i].Pos(), "argument %d: cannot infer type parameter (want %s, got %s)",
					i+1, c.typStr(elemSch), c.typStr(argTypes[i]))
			}
		}
	} else {
		for i := 0; i < len(sig.params) && i < len(argTypes); i++ {
			if argTypes[i] == TypeInvalid {
				continue
			}
			if !c.unify(sig.params[i], argTypes[i], subst) {
				c.error(e.Args[i].Pos(), "argument %d: cannot infer type parameter (want %s, got %s)",
					i+1, c.typStr(sig.params[i]), c.typStr(argTypes[i]))
			}
		}
	}
	for i, tp := range sig.typeParams {
		if _, ok := subst[tp]; !ok {
			c.error(e.Pos(), "cannot infer type argument %s for %s", sig.typeParamNames[i], name)
		}
	}
	params := c.substTypes(sig.params, subst)
	results := c.substTypes(sig.results, subst)
	if sig.variadic && len(params) > 0 {
		fixed := len(params) - 1
		for i := 0; i < fixed && i < len(e.Args); i++ {
			if argTypes[i] != TypeInvalid && !c.assignable(argTypes[i], params[i], e.Args[i]) {
				c.error(e.Args[i].Pos(), "argument %d: want %s, got %s", i+1, c.typStr(params[i]), c.typStr(argTypes[i]))
			}
		}
		elem := ElemOfSlice(c.info, params[fixed])
		for i := fixed; i < len(e.Args); i++ {
			if argTypes[i] != TypeInvalid && !c.assignable(argTypes[i], elem, e.Args[i]) {
				c.error(e.Args[i].Pos(), "argument %d: want %s, got %s", i+1, c.typStr(elem), c.typStr(argTypes[i]))
			}
		}
		c.info.VariadicPacks[e] = &VariadicPack{Slice: params[fixed], Elem: elem, Fixed: fixed}
	} else {
		for i, arg := range e.Args {
			if i < len(params) && argTypes[i] != TypeInvalid && !c.assignable(argTypes[i], params[i], arg) {
				c.error(arg.Pos(), "argument %d: want %s, got %s", i+1, c.typStr(params[i]), c.typStr(argTypes[i]))
			}
		}
	}
	typeArgs := make([]Type, len(sig.typeParams))
	for i, tp := range sig.typeParams {
		typeArgs[i] = subst[tp]
	}
	inst := c.recordMonoInst(pkg, name, sig.decl, typeArgs, params, results)
	if c.info != nil && e != nil {
		c.info.CallInst[e] = inst
	}
	return c.finishCallResult(e, results)
}

func (c *Checker) unify(schematic, concrete Type, subst map[Type]Type) bool {
	if schematic == TypeInvalid || concrete == TypeInvalid {
		return true
	}
	if IsTypeParam(schematic) {
		if prev, ok := subst[schematic]; ok {
			return prev == concrete
		}
		subst[schematic] = concrete
		return true
	}
	if schematic == concrete {
		return true
	}
	if isSlice(schematic) && isSlice(concrete) {
		return c.unify(ElemOfSlice(c.info, schematic), ElemOfSlice(c.info, concrete), subst)
	}
	if isPointer(schematic) && isPointer(concrete) {
		return c.unify(c.elemOfPtr(schematic), c.elemOfPtr(concrete), subst)
	}
	if IsArray(schematic) && IsArray(concrete) {
		sa, ca := c.info.Arrays[schematic], c.info.Arrays[concrete]
		if sa.Len != ca.Len {
			return false
		}
		return c.unify(sa.Elem, ca.Elem, subst)
	}
	if IsMap(schematic) && IsMap(concrete) {
		sm, cm := c.info.Maps[schematic], c.info.Maps[concrete]
		return c.unify(sm.Key, cm.Key, subst) && c.unify(sm.Elem, cm.Elem, subst)
	}
	return false
}

func (c *Checker) substType(t Type, subst map[Type]Type) Type {
	if t2, ok := subst[t]; ok {
		return t2
	}
	if isSlice(t) {
		elem := c.substType(ElemOfSlice(c.info, t), subst)
		if st, ok := c.sliceOf(elem); ok {
			return st
		}
		return TypeInvalid
	}
	if isPointer(t) {
		return c.pointerOf(c.substType(c.elemOfPtr(t), subst))
	}
	if IsArray(t) {
		ai := c.info.Arrays[t]
		return c.arrayOf(ai.Len, c.substType(ai.Elem, subst))
	}
	if IsMap(t) {
		mi := c.info.Maps[t]
		mt, ok := c.mapOf(c.substType(mi.Key, subst), c.substType(mi.Elem, subst))
		if !ok {
			return TypeInvalid
		}
		return mt
	}
	return t
}

func (c *Checker) substTypes(ts []Type, subst map[Type]Type) []Type {
	out := make([]Type, len(ts))
	for i, t := range ts {
		out[i] = c.substType(t, subst)
	}
	return out
}

func (c *Checker) recordMonoInst(pkg, name string, decl *ast.FuncDecl, typeArgs, params, results []Type) *MonoInst {
	key := name
	for _, ta := range typeArgs {
		key += "__" + c.typStr(ta)
	}
	for _, inst := range c.info.Instantiations {
		if inst.Pkg == pkg && inst.Key == key {
			return inst
		}
	}
	inst := &MonoInst{
		Pkg:      pkg,
		Name:     name,
		Decl:     decl,
		TypeArgs: append([]Type(nil), typeArgs...),
		Params:   params,
		Results:  results,
		Key:      key,
	}
	c.info.Instantiations = append(c.info.Instantiations, inst)
	return inst
}

// checkSelectorCall handles X.Name(args): method call or call through a
// function-typed field (not a method value — those use Ident after :=).
func (c *Checker) checkSelectorCall(e *ast.CallExpr, sel *ast.SelectorExpr) Type {
	xt := c.checkExpr(sel.X)
	if xt == TypeInvalid {
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	base := xt
	if isPointer(xt) {
		base = c.elemOfPtr(xt)
	}
	if isStruct(base) {
		si := c.info.Structs[base]
		if mi, ok := si.Methods[sel.Sel.Name]; ok {
			return c.checkMethodCallKnown(e, sel, xt, si, mi)
		}
		for _, f := range si.Fields {
			if f.Name == sel.Sel.Name {
				if c.cur != nil && si.Pkg != c.cur.name && !f.Exported {
					c.error(sel.Sel.Pos(), "field %s.%s is not exported (need வெளி)", si.Name, f.Name)
					for _, arg := range e.Args {
						c.checkExpr(arg)
					}
					return TypeInvalid
				}
				if IsFunc(f.Type) {
					c.record(sel, f.Type)
					return c.checkFuncValueCall(e, f.Type)
				}
				c.error(sel.Pos(), "cannot call non-function field %s", f.Name)
				for _, arg := range e.Args {
					c.checkExpr(arg)
				}
				return TypeInvalid
			}
		}
		c.error(sel.Sel.Pos(), "type %s has no method or field %s", si.Name, sel.Sel.Name)
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	c.error(sel.Pos(), "cannot call method on %s", c.typStr(xt))
	for _, arg := range e.Args {
		c.checkExpr(arg)
	}
	return TypeInvalid
}

func (c *Checker) checkMethodCallKnown(e *ast.CallExpr, sel *ast.SelectorExpr, xt Type, si *StructInfo, mi *MethodInfo) Type {
	if c.cur != nil && si.Pkg != c.cur.name && !mi.Exported {
		c.error(sel.Sel.Pos(), "method %s.%s is not exported (need வெளி)", si.Name, mi.Name)
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	if mi.RecvIsPtr {
		if !isPointer(xt) && !isAddressable(sel.X) {
			c.error(sel.X.Pos(), "cannot call pointer method %s on non-addressable value", mi.Name)
		}
	}
	c.checkCallArgs(e, mi.Params, mi.Variadic, si.Name+"."+mi.Name)
	return c.finishCallResult(e, mi.Results)
}

func (c *Checker) checkFuncValueCall(e *ast.CallExpr, ft Type) Type {
	fi, ok := c.info.Funcs[ft]
	if !ok {
		c.error(e.Pos(), "call of non-function")
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return TypeInvalid
	}
	c.record(e.Fun, ft)
	c.checkCallArgs(e, fi.Params, fi.Variadic, "function value")
	return c.finishCallResult(e, fi.Results)
}
