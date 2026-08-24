// Package parse implements a recursive-descent parser for Niraluli Tamil-0.
package parse

import (
	"fmt"

	"niraluli/internal/ast"
	"niraluli/internal/lex"
	"niraluli/internal/token"
)

// Error is a parse error with position.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// Parser holds parse state.
type Parser struct {
	lex          *lex.Lexer
	tok          token.Token
	errs         []error
	compositeOK  bool // false in if/for/range headers so `x {` is not a composite lit
}

// ParseFile lexes and parses src into an AST.
func ParseFile(filename, src string) (*ast.File, []error) {
	p := &Parser{lex: lex.New(filename, src), compositeOK: true}
	p.next()
	file := p.parseFile()
	file.Path = filename
	errs := append([]error{}, p.lex.Errors()...)
	errs = append(errs, p.errs...)
	return file, errs
}

func (p *Parser) next() {
	p.tok = p.lex.Next()
}

func (p *Parser) errorExpected(msg string) {
	p.errs = append(p.errs, Error{Pos: p.tok.Pos, Msg: msg})
}

// advancePastError consumes one token after a syntax error, unless we are
// already at a sync point. Required inside '{'…'}' loops: returning without
// progress spins forever and grows unbounded AST lists.
func (p *Parser) advancePastError() {
	switch p.tok.Kind {
	case token.EOF, token.RBRACE, token.RPAREN, token.RBRACK, token.SEMICOLON:
		return
	default:
		p.next()
	}
}

func (p *Parser) expect(k token.Kind) token.Token {
	tok := p.tok
	if tok.Kind != k {
		p.errorExpected(fmt.Sprintf("expected %s, got %s", k, tok.Kind))
		p.advancePastError()
		return tok
	}
	p.next()
	return tok
}

func (p *Parser) skipSemis() {
	for p.tok.Kind == token.SEMICOLON {
		p.next()
	}
}

// skipNewlineSemi skips a Go-style ASI semicolon (lit "\n") before ) or }.
func (p *Parser) skipNewlineSemi() {
	if p.tok.Kind == token.SEMICOLON && p.tok.Lit == "\n" {
		p.next()
	}
}

func (p *Parser) parseFile() *ast.File {
	pkg := p.parsePackageClause()
	p.skipSemis()
	var imports []*ast.ImportSpec
	for p.tok.Kind == token.IMPORT {
		imports = append(imports, p.parseImportDecl())
		p.skipSemis()
	}
	var decls []ast.Decl
	for p.tok.Kind != token.EOF {
		p.skipSemis()
		if p.tok.Kind == token.EOF {
			break
		}
		exported := false
		exportPos := p.tok.Pos
		if p.tok.Kind == token.EXPORT {
			exported = true
			p.next()
		}
		switch p.tok.Kind {
		case token.FUNC:
			decls = append(decls, p.parseFuncDecl(exported, exportPos))
		case token.VAR:
			decls = append(decls, p.parseVarDecl(exported, exportPos))
		case token.CONST:
			decls = append(decls, p.parseConstDecl(exported, exportPos))
		case token.TYPE:
			decls = append(decls, p.parseTypeDecl(exported, exportPos))
		case token.IMPORT:
			p.errorExpected("கொணர் must appear before other declarations")
			p.next()
		case token.EXPORT:
			p.errorExpected("வெளி must precede செயல்பாடு, வகை, மாறி, or மாறிலி")
			p.next()
		default:
			if exported {
				p.errorExpected("வெளி must precede செயல்பாடு, வகை, மாறி, or மாறிலி")
			} else {
				p.errorExpected(fmt.Sprintf("expected declaration, got %s", p.tok.Kind))
			}
			p.next()
			if p.tok.Kind == token.EOF {
				break
			}
		}
		p.skipSemis()
	}
	return &ast.File{Package: pkg, Imports: imports, Decls: decls}
}

func (p *Parser) parseImportDecl() *ast.ImportSpec {
	pos := p.tok.Pos
	p.expect(token.IMPORT)
	var alias *ast.Ident
	if p.tok.Kind == token.IDENT {
		alias = p.parseIdent()
	}
	tok := p.tok
	if tok.Kind != token.STRING {
		p.errorExpected("expected import path string")
		p.next()
		return &ast.ImportSpec{TokPos: pos, Alias: alias, Path: &ast.BasicLit{ValuePos: tok.Pos, Kind: token.STRING, Value: ""}}
	}
	p.next()
	return &ast.ImportSpec{
		TokPos: pos,
		Alias:  alias,
		Path:   &ast.BasicLit{ValuePos: tok.Pos, Kind: token.STRING, Value: tok.Lit},
	}
}

func (p *Parser) parsePackageClause() *ast.PackageClause {
	pos := p.tok.Pos
	p.expect(token.PACKAGE)
	name := p.parseIdent()
	return &ast.PackageClause{TokPos: pos, Name: name}
}

func (p *Parser) parseIdent() *ast.Ident {
	tok := p.tok
	if tok.Kind != token.IDENT {
		p.errorExpected(fmt.Sprintf("expected identifier, got %s", tok.Kind))
		p.advancePastError()
		return &ast.Ident{NamePos: tok.Pos, Name: "_"}
	}
	p.next()
	return &ast.Ident{NamePos: tok.Pos, Name: tok.Lit}
}

func (p *Parser) parseType() ast.TypeExpr {
	if p.tok.Kind == token.MUL {
		star := p.tok.Pos
		p.next()
		return &ast.PointerType{Star: star, Elem: p.parseType()}
	}
	if p.tok.Kind == token.ARROW {
		begin := p.tok.Pos
		p.next()
		p.expect(token.CHAN)
		return &ast.ChanType{Begin: begin, Dir: ast.RECV, Elem: p.parseType()}
	}
	if p.tok.Kind == token.CHAN {
		begin := p.tok.Pos
		p.next()
		dir := ast.ChanDir(0)
		if p.tok.Kind == token.ARROW {
			p.next()
			dir = ast.SEND
		}
		return &ast.ChanType{Begin: begin, Dir: dir, Elem: p.parseType()}
	}
	if p.tok.Kind == token.FUNC {
		return p.parseFuncType()
	}
	if p.tok.Kind == token.MAP {
		tokPos := p.tok.Pos
		p.next()
		lbrack := p.expect(token.LBRACK)
		key := p.parseType()
		rbrack := p.expect(token.RBRACK)
		elem := p.parseType()
		return &ast.MapType{
			TokPos: tokPos, Lbrack: lbrack.Pos, Key: key, Rbrack: rbrack.Pos, Elem: elem,
		}
	}
	if p.tok.Kind == token.LBRACK {
		lbrack := p.tok.Pos
		p.next()
		if p.tok.Kind == token.INT {
			lenTok := p.tok
			p.next()
			n := int64(0)
			for _, ch := range lenTok.Lit {
				n = n*10 + int64(ch-'0')
			}
			rbrack := p.expect(token.RBRACK)
			elem := p.parseType()
			return &ast.ArrayType{Lbrack: lbrack, Len: n, LenPos: lenTok.Pos, Rbrack: rbrack.Pos, Elem: elem}
		}
		rbrack := p.expect(token.RBRACK)
		elem := p.parseType()
		return &ast.SliceType{Lbrack: lbrack, Rbrack: rbrack.Pos, Elem: elem}
	}
	tok := p.tok
	switch tok.Kind {
	case token.TYPE_INT, token.TYPE_BOOL, token.TYPE_STRING, token.TYPE_FLOAT,
		token.TYPE_BYTE, token.TYPE_RUNE:
		p.next()
		return &ast.TypeName{TokPos: tok.Pos, Name: tok.Lit}
	case token.IDENT:
		id := p.parseIdent()
		if p.tok.Kind == token.PERIOD {
			p.next()
			name := p.parseIdent()
			return &ast.TypeName{TokPos: id.NamePos, Pkg: id, Name: name.Name}
		}
		return &ast.TypeName{TokPos: id.NamePos, Name: id.Name}
	default:
		p.errorExpected(fmt.Sprintf("expected type, got %s", tok.Kind))
		p.advancePastError()
		return &ast.TypeName{TokPos: tok.Pos, Name: "?"}
	}
}

func startsType(k token.Kind) bool {
	switch k {
	case token.TYPE_INT, token.TYPE_BOOL, token.TYPE_STRING, token.TYPE_FLOAT,
		token.TYPE_BYTE, token.TYPE_RUNE,
		token.LBRACK, token.IDENT, token.MUL, token.MAP, token.FUNC, token.CHAN, token.ARROW:
		return true
	default:
		return false
	}
}

// parseFuncType parses செயல்பாடு "(" [ TypeList ] ")" [ Result ] .
func (p *Parser) parseFuncType() *ast.FuncType {
	funcPos := p.expect(token.FUNC).Pos
	p.expect(token.LPAREN)
	var params []ast.TypeExpr
	variadic := false
	if p.tok.Kind != token.RPAREN && p.tok.Kind != token.SEMICOLON {
		if p.tok.Kind == token.ELLIPSIS {
			p.next()
			variadic = true
		}
		params = append(params, p.parseType())
		for !variadic && p.tok.Kind == token.COMMA {
			p.next()
			if p.tok.Kind == token.ELLIPSIS {
				p.next()
				variadic = true
			}
			params = append(params, p.parseType())
			if variadic {
				break
			}
		}
	}
	p.skipNewlineSemi()
	p.expect(token.RPAREN)
	results := p.parseResultTypes()
	return &ast.FuncType{Func: funcPos, Params: params, Results: results, Variadic: variadic}
}

func (p *Parser) parseTypeDecl(exported bool, exportPos token.Pos) *ast.TypeDecl {
	pos := p.tok.Pos
	if exported {
		pos = exportPos
	}
	p.expect(token.TYPE)
	name := p.parseIdent()
	// வகை Name = Type  (alias)
	if p.tok.Kind == token.ASSIGN {
		p.next()
		return &ast.TypeDecl{
			TokPos:   pos,
			Exported: exported,
			Name:     name,
			Alias:    true,
			Type:     p.parseType(),
		}
	}
	// வகை Name அமைப்பு { … }
	if p.tok.Kind == token.STRUCT {
		stPos := p.tok.Pos
		p.next()
		lbrace := p.expect(token.LBRACE)
		var fields []*ast.Field
		for p.tok.Kind != token.RBRACE && p.tok.Kind != token.EOF {
			p.skipSemis()
			if p.tok.Kind == token.RBRACE || p.tok.Kind == token.EOF {
				break
			}
			fexp := false
			if p.tok.Kind == token.EXPORT {
				fexp = true
				p.next()
			}
			fname := p.parseIdent()
			ftyp := p.parseType()
			fields = append(fields, &ast.Field{Exported: fexp, Name: fname, Type: ftyp})
			p.skipSemis()
		}
		rbrace := p.expect(token.RBRACE)
		return &ast.TypeDecl{
			TokPos:   pos,
			Exported: exported,
			Name:     name,
			Type: &ast.StructType{
				TokPos: stPos,
				Fields: fields,
				Lbrace: lbrace.Pos,
				Rbrace: rbrace.Pos,
			},
		}
	}
	// வகை Name Type  (defined type)
	if startsType(p.tok.Kind) {
		return &ast.TypeDecl{
			TokPos:   pos,
			Exported: exported,
			Name:     name,
			Type:     p.parseType(),
		}
	}
	p.errorExpected("expected அமைப்பு, type, or '=' after வகை name")
	p.advancePastError()
	return &ast.TypeDecl{TokPos: pos, Exported: exported, Name: name, Type: &ast.TypeName{TokPos: p.tok.Pos, Name: "?"}}
}

func (p *Parser) parseFuncDecl(exported bool, exportPos token.Pos) *ast.FuncDecl {
	pos := p.tok.Pos
	if exported {
		pos = exportPos
	}
	p.expect(token.FUNC)
	var recv *ast.Field
	if p.tok.Kind == token.LPAREN {
		// Receiver: ( name Type ) — distinct from plain func params after the name.
		p.next()
		rname := p.parseIdent()
		rtyp := p.parseType()
		p.skipNewlineSemi()
		p.expect(token.RPAREN)
		recv = &ast.Field{Name: rname, Type: rtyp}
	}
	name := p.parseIdent()
	var typeParams []*ast.Ident
	if p.tok.Kind == token.LBRACK {
		typeParams = p.parseTypeParams()
	}
	p.expect(token.LPAREN)
	var params []*ast.Field
	if p.tok.Kind != token.RPAREN && p.tok.Kind != token.SEMICOLON {
		params = p.parseParameterList()
	}
	p.skipNewlineSemi()
	p.expect(token.RPAREN)
	results := p.parseResultTypes()
	body := p.parseBlock()
	return &ast.FuncDecl{
		TokPos:     pos,
		Exported:   exported,
		Recv:       recv,
		Name:       name,
		TypeParams: typeParams,
		Params:     params,
		Results:    results,
		Body:       body,
	}
}

// parseTypeParams parses [ ident { "," ident } ] (unconstrained).
func (p *Parser) parseTypeParams() []*ast.Ident {
	p.expect(token.LBRACK)
	var list []*ast.Ident
	list = append(list, p.parseIdent())
	for p.tok.Kind == token.COMMA {
		p.next()
		list = append(list, p.parseIdent())
	}
	p.skipNewlineSemi()
	p.expect(token.RBRACK)
	return list
}

// parseResultTypes: Type | "(" [ ResultList ] ")" .
// ResultList is either all unnamed types or all named (name Type) fields.
func (p *Parser) parseResultTypes() []*ast.Field {
	if p.tok.Kind == token.LPAREN {
		p.next()
		var list []*ast.Field
		if p.tok.Kind != token.RPAREN && p.tok.Kind != token.SEMICOLON {
			list = p.parseResultList()
		}
		p.skipNewlineSemi()
		p.expect(token.RPAREN)
		return list
	}
	if startsType(p.tok.Kind) {
		return []*ast.Field{{Type: p.parseType()}}
	}
	return nil
}

// parseResultList parses the inside of parenthesized results.
func (p *Parser) parseResultList() []*ast.Field {
	// Named vs unnamed: after a leading Ident, a following type-start means
	// the Ident was a result name; COMMA/RPAREN means it was a type name.
	if p.tok.Kind == token.IDENT {
		id := p.parseIdent()
		if p.tok.Kind == token.COMMA || p.tok.Kind == token.RPAREN || p.tok.Kind == token.SEMICOLON {
			list := []*ast.Field{{Type: &ast.TypeName{TokPos: id.NamePos, Name: id.Name}}}
			for p.tok.Kind == token.COMMA {
				p.next()
				list = append(list, &ast.Field{Type: p.parseType()})
			}
			return list
		}
		if startsType(p.tok.Kind) {
			list := []*ast.Field{{Name: id, Type: p.parseType()}}
			for p.tok.Kind == token.COMMA {
				p.next()
				name := p.parseIdent()
				if !startsType(p.tok.Kind) {
					p.errorExpected("expected type after named result")
				}
				list = append(list, &ast.Field{Name: name, Type: p.parseType()})
			}
			return list
		}
		p.errorExpected("expected type or ',' in result list")
		return []*ast.Field{{Type: &ast.TypeName{TokPos: id.NamePos, Name: id.Name}}}
	}
	// Unnamed: type keywords, [], *, …
	list := []*ast.Field{{Type: p.parseType()}}
	for p.tok.Kind == token.COMMA {
		p.next()
		list = append(list, &ast.Field{Type: p.parseType()})
	}
	return list
}

func (p *Parser) parseParameterList() []*ast.Field {
	var fields []*ast.Field
	for {
		name := p.parseIdent()
		ellipsis := false
		if p.tok.Kind == token.ELLIPSIS {
			p.next()
			ellipsis = true
		}
		typ := p.parseType()
		fields = append(fields, &ast.Field{Name: name, Type: typ, Ellipsis: ellipsis})
		if ellipsis {
			if p.tok.Kind == token.COMMA {
				p.errorExpected("end of parameter list after ...")
				p.next()
			}
			break
		}
		if p.tok.Kind != token.COMMA {
			break
		}
		p.next()
	}
	return fields
}

func (p *Parser) parseVarDecl(exported bool, exportPos token.Pos) *ast.VarDecl {
	pos := p.tok.Pos
	if exported {
		pos = exportPos
	}
	p.expect(token.VAR)
	names := p.parseIdentList()
	typ := p.parseType()
	var values []ast.Expr
	if p.tok.Kind == token.ASSIGN {
		p.next()
		values = p.parseExprList()
	}
	return &ast.VarDecl{TokPos: pos, Exported: exported, Names: names, Type: typ, Values: values}
}

// parseConstDecl parses மாறிலி names [ Type ] = values.
func (p *Parser) parseConstDecl(exported bool, exportPos token.Pos) *ast.ConstDecl {
	pos := p.tok.Pos
	if exported {
		pos = exportPos
	}
	p.expect(token.CONST)
	names := p.parseIdentList()
	var typ ast.TypeExpr
	// Optional type: present when next token is a type start and not "=".
	if p.tok.Kind != token.ASSIGN {
		typ = p.parseType()
	}
	if p.tok.Kind != token.ASSIGN {
		p.errorExpected("மாறிலி requires = initializer")
		return &ast.ConstDecl{TokPos: pos, Exported: exported, Names: names, Type: typ}
	}
	p.next()
	values := p.parseExprList()
	return &ast.ConstDecl{TokPos: pos, Exported: exported, Names: names, Type: typ, Values: values}
}

func (p *Parser) parseIdentList() []*ast.Ident {
	names := []*ast.Ident{p.parseIdent()}
	for p.tok.Kind == token.COMMA {
		p.next()
		names = append(names, p.parseIdent())
	}
	return names
}

func (p *Parser) parseBlock() *ast.BlockStmt {
	lbrace := p.expect(token.LBRACE)
	var list []ast.Stmt
	for p.tok.Kind != token.RBRACE && p.tok.Kind != token.EOF {
		p.skipSemis()
		if p.tok.Kind == token.RBRACE || p.tok.Kind == token.EOF {
			break
		}
		list = append(list, p.parseStmt())
		p.skipSemis()
	}
	rbrace := p.expect(token.RBRACE)
	return &ast.BlockStmt{Lbrace: lbrace.Pos, List: list, Rbrace: rbrace.Pos}
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Kind {
	case token.VAR:
		return p.parseVarDecl(false, p.tok.Pos)
	case token.CONST:
		return p.parseConstDecl(false, p.tok.Pos)
	case token.IF:
		return p.parseIfStmt()
	case token.SWITCH:
		return p.parseSwitchStmt()
	case token.FOR:
		return p.parseForStmt()
	case token.BREAK:
		return p.parseBreakStmt()
	case token.CONTINUE:
		return p.parseContinueStmt()
	case token.RETURN:
		return p.parseReturnStmt()
	case token.DEFER:
		return p.parseDeferStmt()
	case token.GO:
		return p.parseGoStmt()
	case token.SELECT:
		return p.parseSelectStmt()
	case token.LBRACE:
		return p.parseBlock()
	default:
		return p.parseSimpleStmt()
	}
}

func (p *Parser) parseForStmt() ast.Stmt {
	pos := p.tok.Pos
	p.expect(token.FOR)
	if p.tok.Kind == token.LBRACE {
		return &ast.ForStmt{TokPos: pos, Body: p.parseBlock()}
	}
	if p.tok.Kind == token.RANGE {
		return p.parseRangeAfter(pos, nil, false)
	}

	// RangeClause or ForClause / Condition — detect range after IdentList :=| =
	if p.tok.Kind == token.IDENT {
		first := p.parseIdent()
		names := []*ast.Ident{first}
		for p.tok.Kind == token.COMMA {
			p.next()
			names = append(names, p.parseIdent())
		}
		if p.tok.Kind == token.DEFINE || p.tok.Kind == token.ASSIGN {
			define := p.tok.Kind == token.DEFINE
			p.next()
			if p.tok.Kind == token.RANGE {
				return p.parseRangeAfter(pos, names, define)
			}
			// ForClause init: ShortVarDecl or Assign
			var init ast.Stmt
			if define {
				init = &ast.ShortVarDecl{Names: names, Values: p.parseExprList()}
			} else {
				lhs := make([]ast.Expr, len(names))
				for i, n := range names {
					lhs[i] = n
				}
				init = &ast.AssignStmt{LHS: lhs, Values: p.parseExprList()}
			}
			return p.finishForClause(pos, init)
		}
		if len(names) > 1 {
			p.errorExpected("expected := or = after identifier list")
		}
		old := p.compositeOK
		p.compositeOK = false
		x := p.finishExpr(p.parseOperandFromIdent(first))
		p.compositeOK = old
		if p.tok.Kind == token.LBRACE {
			return &ast.ForStmt{TokPos: pos, Cond: x, Body: p.parseBlock()}
		}
		return p.finishForClause(pos, &ast.ExprStmt{X: x})
	}

	if p.tok.Kind == token.SEMICOLON {
		return p.finishForClause(pos, nil)
	}

	// Other expression as condition or init
	first := p.parseSimpleStmt()
	if p.tok.Kind == token.LBRACE {
		es, ok := first.(*ast.ExprStmt)
		if !ok {
			p.errorExpected("சுழல் condition must be an expression")
			return &ast.ForStmt{TokPos: pos, Body: p.parseBlock()}
		}
		return &ast.ForStmt{TokPos: pos, Cond: es.X, Body: p.parseBlock()}
	}
	return p.finishForClause(pos, first)
}

func (p *Parser) parseRangeAfter(pos token.Pos, names []*ast.Ident, define bool) *ast.RangeStmt {
	xtok := p.tok.Pos
	p.expect(token.RANGE)
	x := p.parseExprNoComposite()
	var key, val *ast.Ident
	switch len(names) {
	case 0:
		// bare சுழல் ஒவ்வொரு x
	case 1:
		key = names[0]
	case 2:
		key, val = names[0], names[1]
	default:
		p.errorExpected("ஒவ்வொரு allows at most 2 identifiers")
		key = names[0]
	}
	return &ast.RangeStmt{
		TokPos: pos,
		Key:    key,
		Value:  val,
		Define: define,
		XTok:   xtok,
		X:      x,
		Body:   p.parseBlock(),
	}
}

func (p *Parser) finishForClause(pos token.Pos, init ast.Stmt) *ast.ForStmt {
	var cond ast.Expr
	var post ast.Stmt
	if p.tok.Kind != token.SEMICOLON {
		p.errorExpected("expected ';' or '{' in சுழல் header")
	}
	p.expect(token.SEMICOLON)
	if p.tok.Kind != token.SEMICOLON && p.tok.Kind != token.LBRACE {
		cond = p.parseExprNoComposite()
	}
	p.expect(token.SEMICOLON)
	if p.tok.Kind != token.LBRACE {
		post = p.parseSimpleStmt()
	}
	return &ast.ForStmt{TokPos: pos, Init: init, Cond: cond, Post: post, Body: p.parseBlock()}
}

func (p *Parser) parseBreakStmt() *ast.BreakStmt {
	pos := p.tok.Pos
	p.expect(token.BREAK)
	return &ast.BreakStmt{TokPos: pos}
}

func (p *Parser) parseContinueStmt() *ast.ContinueStmt {
	pos := p.tok.Pos
	p.expect(token.CONTINUE)
	return &ast.ContinueStmt{TokPos: pos}
}

func (p *Parser) parseIfStmt() *ast.IfStmt {
	pos := p.tok.Pos
	p.expect(token.IF)
	init, cond := p.parseIfSwitchHeader(true)
	body := p.parseBlock()
	var elseStmt ast.Stmt
	if p.tok.Kind == token.ELSE {
		p.next()
		if p.tok.Kind == token.IF {
			elseStmt = p.parseIfStmt()
		} else {
			elseStmt = p.parseBlock()
		}
	}
	return &ast.IfStmt{TokPos: pos, Init: init, Cond: cond, Body: body, Else: elseStmt}
}

func (p *Parser) parseSwitchStmt() *ast.SwitchStmt {
	pos := p.tok.Pos
	p.expect(token.SWITCH)
	var init ast.Stmt
	var tag ast.Expr
	if p.tok.Kind != token.LBRACE {
		init, tag = p.parseIfSwitchHeader(false)
	}
	p.expect(token.LBRACE)
	var cases []*ast.CaseClause
	for p.tok.Kind != token.RBRACE && p.tok.Kind != token.EOF {
		p.skipSemis()
		if p.tok.Kind == token.RBRACE || p.tok.Kind == token.EOF {
			break
		}
		cases = append(cases, p.parseCaseClause())
		p.skipSemis()
	}
	p.expect(token.RBRACE)
	return &ast.SwitchStmt{TokPos: pos, Init: init, Tag: tag, Cases: cases}
}

// parseIfSwitchHeader parses [ SimpleStmt ";" ] Expression for எனில்,
// or [ SimpleStmt ";" ] [ Expression ] for திசைவி (needExpr=false allows
// tagless form after init: திசைவி x := 1; { … }).
func (p *Parser) parseIfSwitchHeader(needExpr bool) (init ast.Stmt, expr ast.Expr) {
	old := p.compositeOK
	p.compositeOK = false
	first := p.parseSimpleStmt()
	if p.tok.Kind == token.SEMICOLON {
		init = first
		p.next()
		p.compositeOK = false
		if !needExpr && p.tok.Kind == token.LBRACE {
			p.compositeOK = old
			return init, nil
		}
		expr = p.parseExpr()
		p.compositeOK = old
		return init, expr
	}
	p.compositeOK = old
	es, ok := first.(*ast.ExprStmt)
	if !ok {
		p.errorExpected("expected ; after init, or a condition expression")
		return first, &ast.Ident{Name: "?", NamePos: p.tok.Pos}
	}
	return nil, es.X
}

func (p *Parser) parseCaseClause() *ast.CaseClause {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case token.DEFAULT:
		p.next()
		return &ast.CaseClause{TokPos: pos, Default: true, Body: p.parseBlock()}
	case token.IF:
		p.next()
		list := p.parseExprListNoComposite()
		return &ast.CaseClause{TokPos: pos, List: list, Body: p.parseBlock()}
	default:
		p.errorExpected("expected எனில் or மற்றபடி in திசைவி")
		p.advancePastError()
		return &ast.CaseClause{TokPos: pos, Body: &ast.BlockStmt{}}
	}
}

func (p *Parser) parseExprListNoComposite() []ast.Expr {
	old := p.compositeOK
	p.compositeOK = false
	list := []ast.Expr{p.parseExpr()}
	for p.tok.Kind == token.COMMA {
		p.next()
		list = append(list, p.parseExpr())
	}
	p.compositeOK = old
	return list
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	pos := p.tok.Pos
	p.expect(token.RETURN)
	var results []ast.Expr
	if startsExpr(p.tok.Kind) {
		results = p.parseExprList()
	}
	return &ast.ReturnStmt{TokPos: pos, Results: results}
}

func (p *Parser) parseDeferStmt() *ast.DeferStmt {
	pos := p.tok.Pos
	p.expect(token.DEFER)
	x := p.parseExpr()
	switch e := x.(type) {
	case *ast.CallExpr:
		return &ast.DeferStmt{TokPos: pos, Call: e}
	case *ast.Ident, *ast.SelectorExpr:
		// Bare zero-arg sugar: தள்ளிவை f  ⇒  தள்ளிவை f()
		return &ast.DeferStmt{
			TokPos: pos,
			Call:   &ast.CallExpr{Fun: e, Lparen: e.Pos()},
			Bare:   true,
		}
	default:
		p.errorExpected("தள்ளிவை requires a function call")
		return &ast.DeferStmt{TokPos: pos}
	}
}

func (p *Parser) parseGoStmt() *ast.GoStmt {
	pos := p.tok.Pos
	p.expect(token.GO)
	x := p.parseExpr()
	call, ok := x.(*ast.CallExpr)
	if !ok {
		p.errorExpected("இழை requires a function call")
		return &ast.GoStmt{TokPos: pos}
	}
	return &ast.GoStmt{TokPos: pos, Call: call}
}

func (p *Parser) parseSelectStmt() *ast.SelectStmt {
	pos := p.tok.Pos
	p.expect(token.SELECT)
	p.expect(token.LBRACE)
	var body []*ast.CommClause
	for p.tok.Kind != token.RBRACE && p.tok.Kind != token.EOF {
		p.skipSemis()
		if p.tok.Kind == token.RBRACE || p.tok.Kind == token.EOF {
			break
		}
		body = append(body, p.parseCommClause())
		p.skipSemis()
	}
	p.expect(token.RBRACE)
	return &ast.SelectStmt{TokPos: pos, Body: body}
}

func (p *Parser) parseCommClause() *ast.CommClause {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case token.DEFAULT:
		p.next()
		return &ast.CommClause{TokPos: pos, Default: true, Body: p.parseBlock()}
	case token.IF: // எனில் — same keyword as திசைவி cases / if
		p.next()
		// '{' after the comm starts the clause body — never a composite literal
		// (e.g. எனில் ம := <-ச { ... }).
		old := p.compositeOK
		p.compositeOK = false
		comm := p.parseSelectComm()
		p.compositeOK = old
		return &ast.CommClause{TokPos: pos, Comm: comm, Body: p.parseBlock()}
	default:
		p.errorExpected("expected எனில் or மற்றபடி in தடத்தேர்வு")
		p.advancePastError()
		return &ast.CommClause{TokPos: pos, Body: &ast.BlockStmt{}}
	}
}

// parseSelectComm is parseSimpleStmt without composite-literal '{' (body follows).
func (p *Parser) parseSelectComm() ast.Stmt {
	if p.tok.Kind == token.IDENT {
		first := p.parseIdent()
		if p.tok.Kind == token.COMMA {
			names := []*ast.Ident{first}
			for p.tok.Kind == token.COMMA {
				p.next()
				names = append(names, p.parseIdent())
			}
			switch p.tok.Kind {
			case token.DEFINE:
				p.next()
				return &ast.ShortVarDecl{Names: names, Values: p.parseExprList()}
			case token.ASSIGN:
				p.next()
				lhs := make([]ast.Expr, len(names))
				for i, n := range names {
					lhs[i] = n
				}
				return &ast.AssignStmt{LHS: lhs, Values: p.parseExprList()}
			default:
				p.errorExpected("expected := or = after identifier list")
				return &ast.ShortVarDecl{Names: names, Values: p.parseExprList()}
			}
		}
		x := p.parseOperandFromIdent(first)
		switch p.tok.Kind {
		case token.DEFINE:
			id, ok := x.(*ast.Ident)
			if !ok {
				p.errorExpected(":= requires identifier on the left")
				p.next()
				return &ast.ShortVarDecl{Names: []*ast.Ident{first}, Values: nil}
			}
			p.next()
			return &ast.ShortVarDecl{Names: []*ast.Ident{id}, Values: p.parseExprList()}
		case token.ASSIGN:
			p.next()
			return &ast.AssignStmt{LHS: []ast.Expr{x}, Values: []ast.Expr{p.parseExpr()}}
		case token.ARROW:
			arrow := p.tok.Pos
			p.next()
			return &ast.SendStmt{Chan: p.finishExpr(x), Arrow: arrow, Value: p.parseExpr()}
		default:
			return &ast.ExprStmt{X: p.finishExpr(x)}
		}
	}
	if startsExpr(p.tok.Kind) {
		x := p.parseExpr()
		if p.tok.Kind == token.ASSIGN {
			p.next()
			return &ast.AssignStmt{LHS: []ast.Expr{x}, Values: []ast.Expr{p.parseExpr()}}
		}
		if p.tok.Kind == token.ARROW {
			arrow := p.tok.Pos
			p.next()
			return &ast.SendStmt{Chan: x, Arrow: arrow, Value: p.parseExpr()}
		}
		return &ast.ExprStmt{X: x}
	}
	return &ast.ExprStmt{X: p.parseExpr()}
}

func startsExpr(k token.Kind) bool {
	switch k {
	case token.IDENT, token.INT, token.FLOAT, token.STRING, token.TRUE, token.FALSE, token.NIL,
		token.TYPE_INT, token.TYPE_BOOL, token.TYPE_STRING, token.TYPE_FLOAT,
		token.TYPE_BYTE, token.TYPE_RUNE,
		token.PRINT, token.LEN, token.APPEND, token.MAKE, token.COPY, token.CAP, token.DELETE, token.CLOSE,
		token.PANIC, token.RECOVER,
		token.LPAREN, token.LBRACK, token.MAP, token.CHAN,
		token.SUB, token.NOT, token.MUL, token.AND, token.ARROW:
		return true
	default:
		return false
	}
}

func (p *Parser) parseSimpleStmt() ast.Stmt {
	if p.tok.Kind == token.IDENT {
		first := p.parseIdent()
		if p.tok.Kind == token.COMMA {
			names := []*ast.Ident{first}
			for p.tok.Kind == token.COMMA {
				p.next()
				names = append(names, p.parseIdent())
			}
			switch p.tok.Kind {
			case token.DEFINE:
				p.next()
				vals := p.parseExprListAllowComposite()
				return &ast.ShortVarDecl{Names: names, Values: vals}
			case token.ASSIGN:
				p.next()
				lhs := make([]ast.Expr, len(names))
				for i, n := range names {
					lhs[i] = n
				}
				return &ast.AssignStmt{LHS: lhs, Values: p.parseExprListAllowComposite()}
			default:
				p.errorExpected("expected := or = after identifier list")
				vals := p.parseExprListAllowComposite()
				return &ast.ShortVarDecl{Names: names, Values: vals}
			}
		}
		x := p.parseOperandFromIdent(first)
		switch p.tok.Kind {
		case token.DEFINE:
			id, ok := x.(*ast.Ident)
			if !ok {
				p.errorExpected(":= requires identifier on the left")
				p.next()
				_ = p.parseExprListAllowComposite()
				return &ast.ShortVarDecl{Names: []*ast.Ident{first}, Values: nil}
			}
			p.next()
			return &ast.ShortVarDecl{Names: []*ast.Ident{id}, Values: p.parseExprListAllowComposite()}
		case token.ASSIGN:
			p.next()
			return &ast.AssignStmt{LHS: []ast.Expr{x}, Values: []ast.Expr{p.parseExprAllowComposite()}}
		case token.ARROW:
			arrow := p.tok.Pos
			p.next()
			return &ast.SendStmt{Chan: p.finishExpr(x), Arrow: arrow, Value: p.parseExpr()}
		default:
			return &ast.ExprStmt{X: p.finishExpr(x)}
		}
	}
	if startsExpr(p.tok.Kind) {
		x := p.parseExpr()
		if p.tok.Kind == token.ASSIGN {
			p.next()
			return &ast.AssignStmt{LHS: []ast.Expr{x}, Values: []ast.Expr{p.parseExprAllowComposite()}}
		}
		if p.tok.Kind == token.ARROW {
			arrow := p.tok.Pos
			p.next()
			return &ast.SendStmt{Chan: x, Arrow: arrow, Value: p.parseExpr()}
		}
		return &ast.ExprStmt{X: x}
	}
	return &ast.ExprStmt{X: p.parseExpr()}
}

func (p *Parser) parseExprAllowComposite() ast.Expr {
	old := p.compositeOK
	p.compositeOK = true
	x := p.parseExpr()
	p.compositeOK = old
	return x
}

func (p *Parser) parseExprListAllowComposite() []ast.Expr {
	old := p.compositeOK
	p.compositeOK = true
	list := p.parseExprList()
	p.compositeOK = old
	return list
}

// parseOperandFromIdent finishes call/composite/index/select after a leading identifier.
func (p *Parser) parseOperandFromIdent(id *ast.Ident) ast.Expr {
	var x ast.Expr = id
	if p.tok.Kind == token.LPAREN {
		x = p.parseCall(id, false)
	} else if p.tok.Kind == token.LBRACE && p.compositeOK {
		x = p.parseCompositeLit(&ast.TypeName{TokPos: id.NamePos, Name: id.Name})
	}
	return p.parsePrimarySuffix(x)
}

func (p *Parser) finishExpr(x ast.Expr) ast.Expr {
	x = p.parseFactorLeft(x)
	x = p.parseTermLeft(x)
	x = p.parseComparisonLeft(x)
	x = p.parseEqualityLeft(x)
	return x
}

func (p *Parser) parseExprList() []ast.Expr {
	list := []ast.Expr{p.parseExpr()}
	for p.tok.Kind == token.COMMA {
		p.next()
		list = append(list, p.parseExpr())
	}
	return list
}

func (p *Parser) parseExpr() ast.Expr {
	return p.parseEquality()
}

func (p *Parser) parseExprNoComposite() ast.Expr {
	old := p.compositeOK
	p.compositeOK = false
	x := p.parseExpr()
	p.compositeOK = old
	return x
}

func (p *Parser) parseEquality() ast.Expr {
	return p.parseEqualityLeft(p.parseComparison())
}

func (p *Parser) parseEqualityLeft(x ast.Expr) ast.Expr {
	for p.tok.Kind == token.EQL || p.tok.Kind == token.NEQ {
		op := p.tok
		p.next()
		y := p.parseComparison()
		x = &ast.BinaryExpr{X: x, OpPos: op.Pos, Op: op.Kind, Y: y}
	}
	return x
}

func (p *Parser) parseComparison() ast.Expr {
	return p.parseComparisonLeft(p.parseTerm())
}

func (p *Parser) parseComparisonLeft(x ast.Expr) ast.Expr {
	for p.tok.Kind == token.LSS || p.tok.Kind == token.LEQ || p.tok.Kind == token.GTR || p.tok.Kind == token.GEQ {
		op := p.tok
		p.next()
		y := p.parseTerm()
		x = &ast.BinaryExpr{X: x, OpPos: op.Pos, Op: op.Kind, Y: y}
	}
	return x
}

func (p *Parser) parseTerm() ast.Expr {
	return p.parseTermLeft(p.parseFactor())
}

func (p *Parser) parseTermLeft(x ast.Expr) ast.Expr {
	for p.tok.Kind == token.ADD || p.tok.Kind == token.SUB {
		op := p.tok
		p.next()
		y := p.parseFactor()
		x = &ast.BinaryExpr{X: x, OpPos: op.Pos, Op: op.Kind, Y: y}
	}
	return x
}

func (p *Parser) parseFactor() ast.Expr {
	return p.parseFactorLeft(p.parseUnary())
}

func (p *Parser) parseFactorLeft(x ast.Expr) ast.Expr {
	for p.tok.Kind == token.MUL || p.tok.Kind == token.QUO || p.tok.Kind == token.REM {
		op := p.tok
		p.next()
		y := p.parseUnary()
		x = &ast.BinaryExpr{X: x, OpPos: op.Pos, Op: op.Kind, Y: y}
	}
	return x
}

func (p *Parser) parseUnary() ast.Expr {
	switch p.tok.Kind {
	case token.SUB, token.NOT, token.MUL, token.AND, token.ARROW:
		op := p.tok
		p.next()
		return &ast.UnaryExpr{OpPos: op.Pos, Op: op.Kind, X: p.parseUnary()}
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	return p.parsePrimarySuffix(p.parseOperand())
}

func (p *Parser) parsePrimarySuffix(x ast.Expr) ast.Expr {
	for {
		switch p.tok.Kind {
		case token.LBRACK:
			lbrack := p.tok.Pos
			p.next()
			if p.tok.Kind == token.COLON {
				p.next()
				x = p.finishSlice(x, lbrack, nil)
				continue
			}
			first := p.parseExpr()
			if p.tok.Kind == token.COLON {
				p.next()
				x = p.finishSlice(x, lbrack, first)
				continue
			}
			// f[T, U](…) type-argument list (comma ⇒ instantiation, must be a call).
			if p.tok.Kind == token.COMMA {
				elts := []ast.Expr{first}
				for p.tok.Kind == token.COMMA {
					p.next()
					elts = append(elts, p.parseExpr())
				}
				p.skipNewlineSemi()
				p.expect(token.RBRACK)
				if p.tok.Kind != token.LPAREN {
					p.errorExpected("type argument list must be followed by a call")
					continue
				}
				call := p.parseCallOn(x, false)
				call.TypeArgs = exprsToTypeArgs(elts)
				x = call
				continue
			}
			p.skipNewlineSemi()
			rbrack := p.expect(token.RBRACK)
			x = &ast.IndexExpr{X: x, Lbrack: lbrack, Index: first, Rbrack: rbrack.Pos}
		case token.PERIOD:
			dot := p.tok.Pos
			p.next()
			sel := p.parseIdent()
			x = &ast.SelectorExpr{X: x, Dot: dot, Sel: sel}
			// pkg.Type{ ... } composite literal
			if p.tok.Kind == token.LBRACE && p.compositeOK {
				if s, ok := x.(*ast.SelectorExpr); ok {
					if id, ok := s.X.(*ast.Ident); ok {
						x = p.parseCompositeLit(&ast.TypeName{TokPos: id.NamePos, Pkg: id, Name: s.Sel.Name})
						continue
					}
				}
			}
		case token.LPAREN:
			x = p.parseCallOn(x, false)
		default:
			return x
		}
	}
}

func (p *Parser) parseOperand() ast.Expr {
	switch p.tok.Kind {
	case token.IDENT:
		id := p.parseIdent()
		if p.tok.Kind == token.LPAREN {
			return p.parsePrimarySuffix(p.parseCall(id, false))
		}
		if p.tok.Kind == token.LBRACE && p.compositeOK {
			return p.parsePrimarySuffix(p.parseCompositeLit(&ast.TypeName{TokPos: id.NamePos, Name: id.Name}))
		}
		return id
	case token.TYPE_INT, token.TYPE_BOOL, token.TYPE_STRING, token.TYPE_FLOAT,
		token.TYPE_BYTE, token.TYPE_RUNE:
		// Type name as operand: T(x) or (*T)(x) after unary *.
		tok := p.tok
		p.next()
		id := &ast.Ident{NamePos: tok.Pos, Name: tok.Lit}
		if p.tok.Kind == token.LPAREN {
			return p.parsePrimarySuffix(p.parseCall(id, false))
		}
		return id
	case token.PRINT:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "பதிப்பி"}
		return p.parseCall(id, true)
	case token.LEN:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "நீளம்"}
		return p.parseCall(id, true)
	case token.APPEND:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "சேர்"}
		return p.parseAppendCall(id)
	case token.MAKE:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "ஆக்கு"}
		return p.parseMakeCall(id)
	case token.COPY:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "நகல்"}
		call := p.parseCall(id, false)
		call.Builtin = true
		return call
	case token.CAP:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "திறன்"}
		return p.parseCall(id, true)
	case token.DELETE:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "நீக்கு"}
		call := p.parseCall(id, false)
		call.Builtin = true
		return call
	case token.CLOSE:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "மூடு"}
		call := p.parseCall(id, false)
		call.Builtin = true
		return call
	case token.PANIC:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "அலறு"}
		call := p.parseCall(id, false)
		call.Builtin = true
		return call
	case token.RECOVER:
		pos := p.tok.Pos
		p.next()
		id := &ast.Ident{NamePos: pos, Name: "மீள்"}
		call := p.parseCall(id, false)
		call.Builtin = true
		return call
	case token.MAP:
		typ := p.parseType()
		if p.tok.Kind == token.LBRACE && p.compositeOK {
			return p.parsePrimarySuffix(p.parseCompositeLit(typ))
		}
		p.errorExpected("expected { after map type for composite literal")
		return &ast.Ident{NamePos: typ.Pos(), Name: "?"}
	case token.LBRACK:
		typ := p.parseType()
		switch typ.(type) {
		case *ast.SliceType, *ast.ArrayType:
		default:
			p.errorExpected("composite literal requires slice or array type")
		}
		// []T(x) conversion (Go); otherwise []T{…} composite literal.
		if p.tok.Kind == token.LPAREN {
			if st, ok := typ.(*ast.SliceType); ok {
				return p.parsePrimarySuffix(p.parseCallOn(st, false))
			}
			p.errorExpected("conversion of array type requires parentheses")
		}
		return p.parseCompositeLit(typ)
	case token.INT, token.FLOAT, token.STRING:
		tok := p.tok
		p.next()
		return &ast.BasicLit{ValuePos: tok.Pos, Kind: tok.Kind, Value: tok.Lit}
	case token.TRUE:
		pos := p.tok.Pos
		p.next()
		return &ast.BoolLit{ValuePos: pos, Value: true}
	case token.FALSE:
		pos := p.tok.Pos
		p.next()
		return &ast.BoolLit{ValuePos: pos, Value: false}
	case token.NIL:
		pos := p.tok.Pos
		p.next()
		return &ast.NilLit{ValuePos: pos}
	case token.LPAREN:
		lparen := p.tok.Pos
		p.next()
		x := p.parseExpr()
		p.skipNewlineSemi()
		rparen := p.expect(token.RPAREN)
		return &ast.ParenExpr{Lparen: lparen, X: x, Rparen: rparen.Pos}
	case token.FUNC:
		return p.parsePrimarySuffix(p.parseFuncLit())
	default:
		p.errorExpected(fmt.Sprintf("expected expression, got %s", p.tok.Kind))
		tok := p.tok
		if p.tok.Kind != token.EOF {
			p.next()
		}
		return &ast.Ident{NamePos: tok.Pos, Name: "?"}
	}
}

// parseFuncLit parses செயல்பாடு "(" [ ParameterList ] ")" [ Result ] Block .
func (p *Parser) parseFuncLit() *ast.FuncLit {
	pos := p.expect(token.FUNC).Pos
	p.expect(token.LPAREN)
	var params []*ast.Field
	if p.tok.Kind != token.RPAREN && p.tok.Kind != token.SEMICOLON {
		params = p.parseParameterList()
	}
	p.skipNewlineSemi()
	p.expect(token.RPAREN)
	results := p.parseResultTypes()
	body := p.parseBlock()
	return &ast.FuncLit{Func: pos, Params: params, Results: results, Body: body}
}

func (p *Parser) parseCompositeLit(typ ast.TypeExpr) *ast.CompositeLit {
	lbrace := p.expect(token.LBRACE)
	var elts []ast.Expr
	if p.tok.Kind != token.RBRACE && p.tok.Kind != token.SEMICOLON {
		elts = p.parseLiteralElements()
	}
	p.skipNewlineSemi()
	rbrace := p.expect(token.RBRACE)
	return &ast.CompositeLit{Type: typ, Lbrace: lbrace.Pos, Elts: elts, Rbrace: rbrace.Pos}
}

func (p *Parser) parseLiteralElements() []ast.Expr {
	var list []ast.Expr
	for {
		list = append(list, p.parseLiteralElement())
		if p.tok.Kind != token.COMMA {
			break
		}
		p.next()
		if p.tok.Kind == token.RBRACE {
			break
		}
	}
	return list
}

func (p *Parser) parseLiteralElement() ast.Expr {
	// Parse an expression; if followed by ":", treat as keyed element
	// (struct field name or map key).
	x := p.parseExpr()
	if p.tok.Kind == token.COLON {
		colon := p.tok.Pos
		p.next()
		return &ast.KeyValueExpr{Key: x, Colon: colon, Value: p.parseExpr()}
	}
	return x
}

func (p *Parser) parseCall(fun *ast.Ident, builtin bool) *ast.CallExpr {
	return p.parseCallOn(fun, builtin)
}

func (p *Parser) parseCallOn(fun ast.Expr, builtin bool) *ast.CallExpr {
	lparen := p.expect(token.LPAREN)
	var args []ast.Expr
	if builtin {
		args = []ast.Expr{p.parseExpr()}
	} else if p.tok.Kind != token.RPAREN && p.tok.Kind != token.SEMICOLON {
		args = p.parseExprList()
	}
	p.skipNewlineSemi()
	rparen := p.expect(token.RPAREN)
	return &ast.CallExpr{Fun: fun, Lparen: lparen.Pos, Args: args, Rparen: rparen.Pos, Builtin: builtin}
}

// finishSlice parses the remainder of a slice expression after the first ":".
// Forms: [low:high], [low:], [:high], [:], [low:high:max], [:high:max].
func (p *Parser) finishSlice(x ast.Expr, lbrack token.Pos, low ast.Expr) ast.Expr {
	var high, max ast.Expr
	slice3 := false
	if p.tok.Kind != token.RBRACK && p.tok.Kind != token.SEMICOLON && p.tok.Kind != token.COLON {
		high = p.parseExpr()
	}
	if p.tok.Kind == token.COLON {
		if high == nil {
			p.errorExpected("middle index required in three-index slice")
		}
		p.next()
		max = p.parseExpr()
		slice3 = true
	}
	p.skipNewlineSemi()
	rbrack := p.expect(token.RBRACK)
	return &ast.SliceExpr{
		X: x, Lbrack: lbrack, Low: low, High: high, Max: max, Slice3: slice3, Rbrack: rbrack.Pos,
	}
}

func (p *Parser) parseAppendCall(fun *ast.Ident) *ast.CallExpr {
	lparen := p.expect(token.LPAREN)
	a0 := p.parseExpr()
	args := []ast.Expr{a0}
	if p.tok.Kind == token.COMMA {
		p.next()
		args = append(args, p.parseExprList()...)
	}
	p.skipNewlineSemi()
	rparen := p.expect(token.RPAREN)
	return &ast.CallExpr{Fun: fun, Lparen: lparen.Pos, Args: args, Rparen: rparen.Pos, Builtin: true}
}

// exprsToTypeArgs converts index-list expressions into type expressions for f[T,U](…).
func exprsToTypeArgs(elts []ast.Expr) []ast.TypeExpr {
	out := make([]ast.TypeExpr, 0, len(elts))
	for _, el := range elts {
		if te := exprAsType(el); te != nil {
			out = append(out, te)
		} else {
			out = append(out, &ast.TypeName{TokPos: el.Pos(), Name: "?"})
		}
	}
	return out
}

func exprAsType(e ast.Expr) ast.TypeExpr {
	switch e := e.(type) {
	case *ast.Ident:
		return &ast.TypeName{TokPos: e.NamePos, Name: e.Name}
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			return &ast.TypeName{TokPos: id.NamePos, Pkg: id, Name: e.Sel.Name}
		}
	case *ast.ParenExpr:
		return exprAsType(e.X)
	case *ast.UnaryExpr:
		if e.Op == token.MUL {
			if elem := exprAsType(e.X); elem != nil {
				return &ast.PointerType{Star: e.OpPos, Elem: elem}
			}
		}
	}
	return nil
}

// parseMakeCall parses ஆக்கு for slices (Type, len [, cap]) or maps (Type [, hint]).
func (p *Parser) parseMakeCall(fun *ast.Ident) *ast.CallExpr {
	lparen := p.expect(token.LPAREN)
	typ := p.parseType()
	var args []ast.Expr
	if p.tok.Kind == token.COMMA {
		p.next()
		args = append(args, p.parseExpr())
		if p.tok.Kind == token.COMMA {
			p.next()
			args = append(args, p.parseExpr())
		}
	}
	p.skipNewlineSemi()
	rparen := p.expect(token.RPAREN)
	return &ast.CallExpr{
		Fun: fun, Lparen: lparen.Pos, TypeArg: typ, Args: args, Rparen: rparen.Pos, Builtin: true,
	}
}
