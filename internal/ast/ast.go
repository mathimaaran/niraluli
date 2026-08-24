// Package ast defines Niraluli abstract syntax trees.
package ast

import (
	"fmt"

	"niraluli/internal/token"
)

// Node is implemented by all AST nodes.
type Node interface {
	Pos() token.Pos
}

// File is a parsed source file.
type File struct {
	Path    string // source path (set by parser/load)
	Package *PackageClause
	Imports []*ImportSpec
	Decls   []Decl
}

// ImportSpec is: கொணர் [ Alias ] "path"
type ImportSpec struct {
	TokPos token.Pos
	Alias  *Ident    // optional local name; "_" = blank
	Path   *BasicLit // STRING
	Name   string    // local qualifier used in this file (set by load)
	Pkg    string    // real தொகுப்பு name (set by load)
}

func (s *ImportSpec) Pos() token.Pos { return s.TokPos }

func (f *File) Pos() token.Pos {
	if f.Package != nil {
		return f.Package.Pos()
	}
	return token.Pos{}
}

// PackageClause is: தொகுப்பு Name
type PackageClause struct {
	TokPos token.Pos
	Name   *Ident
}

func (p *PackageClause) Pos() token.Pos { return p.TokPos }

// Decl is a top-level declaration.
type Decl interface {
	Node
	declNode()
}

// FuncDecl is a function or method declaration.
type FuncDecl struct {
	TokPos     token.Pos
	Exported   bool // வெளி
	Recv       *Field // optional method receiver
	Name       *Ident
	TypeParams []*Ident // unconstrained type parameters (Tamil-0.40); empty = non-generic
	Params     []*Field
	Results    []*Field // empty = void; Name==nil → unnamed type; all named or all unnamed
	Body       *BlockStmt
}

func (d *FuncDecl) Pos() token.Pos { return d.TokPos }
func (d *FuncDecl) declNode()      {}

// TypeDecl is வகை Name TypeLit | வகை Name = Type | வகை Name Type.
type TypeDecl struct {
	TokPos   token.Pos
	Exported bool // வெளி
	Name     *Ident
	Alias    bool     // true for வகை Name = Type
	Type     TypeExpr // *StructType, or underlying/aliased type
}

func (d *TypeDecl) Pos() token.Pos { return d.TokPos }
func (d *TypeDecl) declNode()      {}

// Field is a parameter, result, or struct field.
// Name may be nil for unnamed results. Exported applies to struct fields only.
// Ellipsis marks a variadic parameter (...T); Type is the element type T.
type Field struct {
	Exported bool // வெளி (struct fields)
	Name     *Ident
	Type     TypeExpr
	Ellipsis bool // ...T (function/method parameters only; must be last)
}

func (f *Field) Pos() token.Pos {
	if f.Name != nil {
		return f.Name.Pos()
	}
	return token.Pos{}
}

// TypeExpr is a type in source (named or slice).
type TypeExpr interface {
	Node
	typeExpr()
}

// TypeName is முழுஎண், நிலை, சரம், Name, or Pkg.Name.
type TypeName struct {
	TokPos token.Pos
	Pkg    *Ident // optional package qualifier
	Name   string
}

func (t *TypeName) Pos() token.Pos { return t.TokPos }
func (t *TypeName) typeExpr()      {}

// SliceType is []Elem.
type SliceType struct {
	Lbrack token.Pos
	Rbrack token.Pos
	Elem   TypeExpr
}

func (t *SliceType) Pos() token.Pos { return t.Lbrack }
func (t *SliceType) typeExpr()      {}
func (t *SliceType) exprNode()      {} // also Expr: []T(x) conversion

// ArrayType is [Len]Elem (Len is a non-negative integer literal).
type ArrayType struct {
	Lbrack token.Pos
	Len    int64
	LenPos token.Pos
	Rbrack token.Pos
	Elem   TypeExpr
}

func (t *ArrayType) Pos() token.Pos { return t.Lbrack }
func (t *ArrayType) typeExpr()      {}

// PointerType is *Elem.
type PointerType struct {
	Star token.Pos
	Elem TypeExpr
}

func (t *PointerType) Pos() token.Pos { return t.Star }
func (t *PointerType) typeExpr()      {}

// MapType is அகராதி[Key]Elem.
type MapType struct {
	TokPos token.Pos
	Lbrack token.Pos
	Key    TypeExpr
	Rbrack token.Pos
	Elem   TypeExpr
}

func (t *MapType) Pos() token.Pos { return t.TokPos }
func (t *MapType) typeExpr()      {}

// ChanDir is the channel direction (Go-like).
type ChanDir int

const (
	SEND ChanDir = 1 << iota
	RECV
)

// ChanType is தடம் T, தடம்<- T, or <-தடம் T.
type ChanType struct {
	Begin token.Pos // CHAN or ARROW
	Dir   ChanDir   // 0 = bidirectional; SEND; RECV
	Elem  TypeExpr
}

func (t *ChanType) Pos() token.Pos { return t.Begin }
func (t *ChanType) typeExpr()      {}

// FuncType is செயல்பாடு(TypeList) [Result] (function type; Tamil-0.44).
// If Variadic, Params[len-1] is the element type of ...T.
type FuncType struct {
	Func     token.Pos
	Params   []TypeExpr // unnamed parameter types
	Results  []*Field   // same shape as FuncDecl.Results
	Variadic bool
}

func (t *FuncType) Pos() token.Pos { return t.Func }
func (t *FuncType) typeExpr()      {}

// StructType is அமைப்பு { fields }
type StructType struct {
	TokPos token.Pos
	Fields []*Field
	Lbrace token.Pos
	Rbrace token.Pos
}

func (t *StructType) Pos() token.Pos { return t.TokPos }
func (t *StructType) typeExpr()      {}

// TypeString returns a display form for a type expression.
func TypeString(t TypeExpr) string {
	switch t := t.(type) {
	case *TypeName:
		if t.Pkg != nil {
			return t.Pkg.Name + "." + t.Name
		}
		return t.Name
	case *SliceType:
		return "[]" + TypeString(t.Elem)
	case *ArrayType:
		return fmt.Sprintf("[%d]%s", t.Len, TypeString(t.Elem))
	case *PointerType:
		return "*" + TypeString(t.Elem)
	case *MapType:
		return "அகராதி[" + TypeString(t.Key) + "]" + TypeString(t.Elem)
	case *ChanType:
		switch t.Dir {
		case SEND:
			return "தடம்<-" + TypeString(t.Elem)
		case RECV:
			return "<-தடம்" + TypeString(t.Elem)
		default:
			return "தடம் " + TypeString(t.Elem)
		}
	case *FuncType:
		s := "செயல்பாடு("
		for i, p := range t.Params {
			if i > 0 {
				s += ", "
			}
			s += TypeString(p)
		}
		s += ")"
		if len(t.Results) == 1 && t.Results[0].Name == nil {
			s += " " + TypeString(t.Results[0].Type)
		} else if len(t.Results) > 0 {
			s += " ("
			for i, r := range t.Results {
				if i > 0 {
					s += ", "
				}
				if r.Name != nil {
					s += r.Name.Name + " "
				}
				s += TypeString(r.Type)
			}
			s += ")"
		}
		return s
	case *StructType:
		return "அமைப்பு"
	default:
		return "?"
	}
}

// VarDecl is மாறி names Type [ = values ]
type VarDecl struct {
	TokPos   token.Pos
	Exported bool // வெளி (top-level only; unused until package vars)
	Names    []*Ident
	Type     TypeExpr
	Values   []Expr
}

func (d *VarDecl) Pos() token.Pos { return d.TokPos }
func (d *VarDecl) declNode()      {}
func (d *VarDecl) stmtNode()      {}

// Stmt is a statement.
type Stmt interface {
	Node
	stmtNode()
}

// BlockStmt is { stmts }
type BlockStmt struct {
	Lbrace token.Pos
	List   []Stmt
	Rbrace token.Pos
}

func (b *BlockStmt) Pos() token.Pos { return b.Lbrace }
func (b *BlockStmt) stmtNode()      {}

// IfStmt is எனில் [ Init ; ] Cond Body [ இல்லையேல் Else ]
type IfStmt struct {
	TokPos token.Pos
	Init   Stmt // optional
	Cond   Expr
	Body   *BlockStmt
	Else   Stmt
}

func (s *IfStmt) Pos() token.Pos { return s.TokPos }
func (s *IfStmt) stmtNode()      {}

// SwitchStmt is திசைவி [ Init ; ] [ Tag ] { CaseClause… }
type SwitchStmt struct {
	TokPos token.Pos
	Init   Stmt // optional
	Tag    Expr // optional; nil = tagless (boolean cases)
	Cases  []*CaseClause
}

func (s *SwitchStmt) Pos() token.Pos { return s.TokPos }
func (s *SwitchStmt) stmtNode()      {}

// CaseClause is எனில் ExprList Block  or  மற்றபடி Block.
type CaseClause struct {
	TokPos  token.Pos
	List    []Expr // empty when Default
	Body    *BlockStmt
	Default bool
}

func (c *CaseClause) Pos() token.Pos { return c.TokPos }

// ForStmt is சுழல் [ Init ; Cond ; Post ] Body  or  சுழல் Cond Body  or  சுழல் Body
type ForStmt struct {
	TokPos token.Pos
	Init   Stmt
	Cond   Expr
	Post   Stmt
	Body   *BlockStmt
}

func (s *ForStmt) Pos() token.Pos { return s.TokPos }
func (s *ForStmt) stmtNode()      {}

// RangeStmt is சுழல் Key[, Value] :=| = ஒவ்வொரு X { Body }
type RangeStmt struct {
	TokPos token.Pos
	Key    *Ident // required; may be "_"
	Value  *Ident // optional; may be "_"
	Define bool   // true for :=, false for =
	XTok   token.Pos
	X      Expr
	Body   *BlockStmt
}

func (s *RangeStmt) Pos() token.Pos { return s.TokPos }
func (s *RangeStmt) stmtNode()      {}

// BreakStmt is முறி
type BreakStmt struct {
	TokPos token.Pos
}

func (s *BreakStmt) Pos() token.Pos { return s.TokPos }
func (s *BreakStmt) stmtNode()      {}

// ContinueStmt is தொடர்
type ContinueStmt struct {
	TokPos token.Pos
}

func (s *ContinueStmt) Pos() token.Pos { return s.TokPos }
func (s *ContinueStmt) stmtNode()      {}

// ReturnStmt is திருப்பு [ ExpressionList ]
type ReturnStmt struct {
	TokPos  token.Pos
	Results []Expr
}

func (s *ReturnStmt) Pos() token.Pos { return s.TokPos }
func (s *ReturnStmt) stmtNode()      {}

// DeferStmt is தள்ளிவை CallExpr, or a bare name/selector rewritten as a zero-arg call.
type DeferStmt struct {
	TokPos token.Pos
	Call   *CallExpr
	Bare   bool // true if source was தள்ளிவை f (sugar for f())
}

func (s *DeferStmt) Pos() token.Pos { return s.TokPos }
func (s *DeferStmt) stmtNode()      {}

// GoStmt is இழை CallExpr.
type GoStmt struct {
	TokPos token.Pos
	Call   *CallExpr
}

func (s *GoStmt) Pos() token.Pos { return s.TokPos }
func (s *GoStmt) stmtNode()      {}

// SendStmt is Chan <- Value.
type SendStmt struct {
	Chan  Expr
	Arrow token.Pos
	Value Expr
}

func (s *SendStmt) Pos() token.Pos {
	if s.Chan != nil {
		return s.Chan.Pos()
	}
	return s.Arrow
}
func (s *SendStmt) stmtNode() {}

// SelectStmt is தடத்தேர்வு { CommClause… }.
type SelectStmt struct {
	TokPos token.Pos
	Body   []*CommClause
}

func (s *SelectStmt) Pos() token.Pos { return s.TokPos }
func (s *SelectStmt) stmtNode()      {}

// CommClause is எனில் Comm Block or மற்றபடி Block (inside தடத்தேர்வு).
// Comm is SendStmt, UnaryExpr{ARROW, …} recv, or Assign/ShortVar of recv.
type CommClause struct {
	TokPos  token.Pos
	Comm    Stmt // nil when Default; else SendStmt, AssignStmt, ShortVarDecl, or ExprStmt(recv)
	Body    *BlockStmt
	Default bool
}

func (c *CommClause) Pos() token.Pos { return c.TokPos }

// AssignStmt is LHS = Value, or LHS0, LHS1, … = call (multi-assign unpack).
type AssignStmt struct {
	LHS    []Expr
	Values []Expr
}

func (s *AssignStmt) Pos() token.Pos {
	if len(s.LHS) > 0 && s.LHS[0] != nil {
		return s.LHS[0].Pos()
	}
	return token.Pos{}
}
func (s *AssignStmt) stmtNode() {}

// ShortVarDecl is names := values
type ShortVarDecl struct {
	Names  []*Ident
	Values []Expr
}

func (s *ShortVarDecl) Pos() token.Pos {
	if len(s.Names) > 0 {
		return s.Names[0].Pos()
	}
	return token.Pos{}
}
func (s *ShortVarDecl) stmtNode() {}

// ExprStmt is a bare expression (e.g. call).
type ExprStmt struct {
	X Expr
}

func (s *ExprStmt) Pos() token.Pos {
	if s.X != nil {
		return s.X.Pos()
	}
	return token.Pos{}
}
func (s *ExprStmt) stmtNode() {}

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// Ident is a name.
type Ident struct {
	NamePos token.Pos
	Name    string
}

func (id *Ident) Pos() token.Pos { return id.NamePos }
func (id *Ident) exprNode()      {}

// BasicLit is INT or STRING.
type BasicLit struct {
	ValuePos token.Pos
	Kind     token.Kind
	Value    string
}

func (b *BasicLit) Pos() token.Pos { return b.ValuePos }
func (b *BasicLit) exprNode()      {}

// BoolLit is மெய் or பொய்.
type BoolLit struct {
	ValuePos token.Pos
	Value    bool
}

func (b *BoolLit) Pos() token.Pos { return b.ValuePos }
func (b *BoolLit) exprNode()      {}

// NilLit is இன்மை (nil pointer).
type NilLit struct {
	ValuePos token.Pos
}

func (n *NilLit) Pos() token.Pos { return n.ValuePos }
func (n *NilLit) exprNode()      {}

// UnaryExpr is Op X
type UnaryExpr struct {
	OpPos token.Pos
	Op    token.Kind
	X     Expr
}

func (u *UnaryExpr) Pos() token.Pos { return u.OpPos }
func (u *UnaryExpr) exprNode()      {}

// BinaryExpr is X Op Y
type BinaryExpr struct {
	X     Expr
	OpPos token.Pos
	Op    token.Kind
	Y     Expr
}

func (b *BinaryExpr) Pos() token.Pos {
	if b.X != nil {
		return b.X.Pos()
	}
	return b.OpPos
}
func (b *BinaryExpr) exprNode() {}

// CallExpr is Fun(Args) — user function or builtin.
type CallExpr struct {
	Fun        Expr
	Lparen     token.Pos
	TypeArg    TypeExpr   // ஆக்கு([]T, …): type is first argument
	TypeArgs   []TypeExpr // f[T,U](…) generic instantiation (Tamil-0.41)
	Args       []Expr
	Rparen     token.Pos
	Builtin    bool // true for பதிப்பி / நீளம் / ஆக்கு / …
	Conversion bool // true for T(x) type conversion
}

func (c *CallExpr) Pos() token.Pos {
	if c.Fun != nil {
		return c.Fun.Pos()
	}
	return c.Lparen
}
func (c *CallExpr) exprNode() {}

// ParenExpr is (X)
type ParenExpr struct {
	Lparen token.Pos
	X      Expr
	Rparen token.Pos
}

func (p *ParenExpr) Pos() token.Pos { return p.Lparen }
func (p *ParenExpr) exprNode()      {}

// CompositeLit is []T{ elts }
type CompositeLit struct {
	Type   TypeExpr
	Lbrace token.Pos
	Elts   []Expr
	Rbrace token.Pos
}

func (c *CompositeLit) Pos() token.Pos {
	if c.Type != nil {
		return c.Type.Pos()
	}
	return c.Lbrace
}
func (c *CompositeLit) exprNode() {}

// IndexExpr is X[Index]
type IndexExpr struct {
	X      Expr
	Lbrack token.Pos
	Index  Expr
	Rbrack token.Pos
}

func (i *IndexExpr) Pos() token.Pos {
	if i.X != nil {
		return i.X.Pos()
	}
	return i.Lbrack
}
func (i *IndexExpr) exprNode() {}

// SliceExpr is X[Low:High] or X[Low:High:Max] (bounds may be nil except High when Max set).
type SliceExpr struct {
	X      Expr
	Lbrack token.Pos
	Low    Expr // optional
	High   Expr // optional; required when Max != nil
	Max    Expr // optional; three-index form when set
	Slice3 bool // true for xs[i:j:k] / xs[:j:k]
	Rbrack token.Pos
}

func (s *SliceExpr) Pos() token.Pos {
	if s.X != nil {
		return s.X.Pos()
	}
	return s.Lbrack
}
func (s *SliceExpr) exprNode() {}

// SelectorExpr is X.Sel
type SelectorExpr struct {
	X   Expr
	Dot token.Pos
	Sel *Ident
}

func (s *SelectorExpr) Pos() token.Pos {
	if s.X != nil {
		return s.X.Pos()
	}
	return s.Dot
}
func (s *SelectorExpr) exprNode() {}

// FuncLit is செயல்பாடு(params) [results] { body } (Tamil-0.45).
type FuncLit struct {
	Func    token.Pos
	Params  []*Field
	Results []*Field
	Body    *BlockStmt
}

func (f *FuncLit) Pos() token.Pos { return f.Func }
func (f *FuncLit) exprNode()      {}

// KeyValueExpr is key: value inside a composite literal.
// Key is a field name (*Ident) for structs, or any expression for maps.
type KeyValueExpr struct {
	Key   Expr
	Colon token.Pos
	Value Expr
}

func (k *KeyValueExpr) Pos() token.Pos {
	if k.Key != nil {
		return k.Key.Pos()
	}
	return k.Colon
}
func (k *KeyValueExpr) exprNode() {}
