package check

import (
	"strconv"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/token"
)

// FmtPlan describes a வடிவம்.வடிவமை call with a constant format (Tamil-0.77).
// Args are stringified at emit time; Format is normalized to %s verbs.
type FmtPlan struct {
	Format string // original format (emit may normalize)
	Args   []ast.Expr
	Verbs  []rune // one per arg verb: 's','d','v','g','t'
}

func (c *Checker) tryCheckFmtSprintf(e *ast.CallExpr, imp *pkgState, sel *ast.SelectorExpr) (Type, bool) {
	if e == nil || imp == nil || sel == nil || sel.Sel == nil {
		return TypeInvalid, false
	}
	if imp.name != "வடிவம்" || sel.Sel.Name != "வடிவமை" {
		return TypeInvalid, false
	}
	if len(e.Args) < 1 {
		return TypeInvalid, false
	}
	format, ok := c.constStringLit(e.Args[0])
	if !ok {
		return TypeInvalid, false
	}
	verbs := parseFmtVerbs(format)
	hasTyped := false
	for _, v := range verbs {
		if v != 's' {
			hasTyped = true
			break
		}
	}
	// Pure %s (or no verbs) can use the normal ...சரம் path.
	if !hasTyped {
		return TypeInvalid, false
	}

	c.checkExpr(e.Args[0])
	args := e.Args[1:]
	for i, arg := range args {
		t := c.checkExpr(arg)
		if i < len(verbs) {
			if !c.fmtVerbAssignable(verbs[i], t) {
				c.error(arg.Pos(), "format verb %%%c cannot use argument type %s", verbs[i], c.typStr(t))
			}
		}
	}
	if c.info != nil {
		if c.info.FmtPlans == nil {
			c.info.FmtPlans = map[*ast.CallExpr]*FmtPlan{}
		}
		c.info.FmtPlans[e] = &FmtPlan{
			Format: format,
			Args:   args,
			Verbs:  verbs,
		}
	}
	return TypeString, true
}

func (c *Checker) constStringLit(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return e.Value, true
		}
		return s, true
	case *ast.ParenExpr:
		return c.constStringLit(e.X)
	default:
		if c.info != nil {
			if cv, ok := c.info.ConstExprs[e]; ok && cv.Kind == ConstString {
				return cv.Str, true
			}
		}
		return "", false
	}
}

func parseFmtVerbs(format string) []rune {
	var verbs []rune
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		if i+1 >= len(format) {
			break
		}
		next := format[i+1]
		if next == '%' {
			i++
			continue
		}
		switch next {
		case 's', 'd', 'v', 'g', 't':
			verbs = append(verbs, rune(next))
			i++
		default:
			// unknown verb: leave literal; no arg consumed
		}
	}
	return verbs
}

func (c *Checker) fmtVerbAssignable(verb rune, t Type) bool {
	t = c.underlying(t)
	switch verb {
	case 's':
		return t == TypeString || t == TypeInvalid
	case 'd':
		return c.isInteger(t) || t == TypeInvalid
	case 'g':
		return t == TypeFloat || c.isInteger(t) || t == TypeInvalid
	case 't':
		return t == TypeBool || t == TypeInvalid
	case 'v':
		return t == TypeString || t == TypeBool || t == TypeFloat || c.isInteger(t) || t == TypeInvalid
	default:
		return false
	}
}

// NormalizeFmtFormat rewrites %d/%v/%g/%t to %s for the string-arg sprintf runtime.
func NormalizeFmtFormat(format string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			next := format[i+1]
			if next == '%' {
				b.WriteString("%%")
				i++
				continue
			}
			switch next {
			case 's', 'd', 'v', 'g', 't':
				b.WriteString("%s")
				i++
				continue
			}
		}
		b.WriteByte(format[i])
	}
	return b.String()
}

// IsIntegerLike reports whether t is any integer width (including byte/rune).
func IsIntegerLike(t Type) bool {
	switch t {
	case TypeInt, TypeByte, TypeRune,
		TypeInt8, TypeInt16, TypeInt32, TypeInt64,
		TypeUint8, TypeUint16, TypeUint32, TypeUint64:
		return true
	}
	return false
}
