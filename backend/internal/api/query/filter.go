package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InputError is a client-caused query problem ($filter, sort, or cursor): the
// handler maps it to 400 with Message verbatim in the {"error": ...} envelope.
// Its messages name the offending token and, where useful, suggest a fix.
// Anything else the runtime returns is an internal fault (500).
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

// filterErr builds an InputError for a $filter problem (messages say "$filter").
func filterErr(format string, a ...any) error {
	return &InputError{Message: fmt.Sprintf(format, a...)}
}

// inputErr builds an InputError for a sort/cursor/other query problem.
func inputErr(format string, a ...any) error {
	return &InputError{Message: fmt.Sprintf(format, a...)}
}

// Compile turns an OData $filter string into a SQL boolean expression and its
// bound arguments. The expression has no leading WHERE/AND — the caller ANDs it
// onto the workspace/soft-delete predicates. Placeholders start at startParam
// ($1 is conventionally the workspace id), and every operand is a bound
// parameter, so a filter can never inject SQL. An empty filter yields "".
func Compile(filter string, fs *FieldSet, startParam int) (string, []any, error) {
	if strings.TrimSpace(filter) == "" {
		return "", nil, nil
	}
	toks, err := lex(filter)
	if err != nil {
		return "", nil, err
	}
	p := &parser{toks: toks}
	ast, err := p.parse()
	if err != nil {
		return "", nil, err
	}
	b := &binder{fs: fs, param: startParam}
	sql, err := b.bind(ast)
	if err != nil {
		return "", nil, err
	}
	return sql, b.args, nil
}

// --- lexer ---

type tokKind int

const (
	tIdent tokKind = iota // field, keyword (and/or/not/eq/.../null/true/false), or function name
	tStr                  // 'quoted string'
	tNum                  // unquoted number or datetime literal
	tLParen
	tRParen
	tComma
	tEOF
)

type token struct {
	kind tokKind
	val  string
	pos  int // 1-based rune offset, for error messages
}

func lex(s string) ([]token, error) {
	var out []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			out = append(out, token{tLParen, "(", i + 1})
			i++
		case c == ')':
			out = append(out, token{tRParen, ")", i + 1})
			i++
		case c == ',':
			out = append(out, token{tComma, ",", i + 1})
			i++
		case c == '\'':
			start := i
			i++
			var sb strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' { // '' -> literal '
						sb.WriteByte('\'')
						i += 2
						continue
					}
					closed = true
					i++
					break
				}
				sb.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, filterErr("unterminated string literal in $filter (missing closing ')")
			}
			out = append(out, token{tStr, sb.String(), start + 1})
		case isIdentStart(c):
			start := i
			for i < len(s) && isIdentChar(s[i]) {
				i++
			}
			out = append(out, token{tIdent, s[start:i], start + 1})
		case c >= '0' && c <= '9' || (c == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'):
			start := i
			i++ // first digit or leading '-'
			for i < len(s) && isNumChar(s[i]) {
				i++
			}
			out = append(out, token{tNum, s[start:i], start + 1})
		default:
			return nil, filterErr("unexpected character %q at position %d in $filter", string(c), i+1)
		}
	}
	out = append(out, token{tEOF, "", len(s) + 1})
	return out, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// isNumChar covers the characters of a number or an RFC3339 datetime
// (2026-01-01T00:00:00Z, +02:00 offsets, fractional seconds).
func isNumChar(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-' || c == ':' || c == '.' || c == '+' ||
		c == 'T' || c == 'Z' || c == 'z'
}

// --- AST ---

type node interface{}

type boolNode struct {
	op    string // "and" | "or"
	items []node
}
type notNode struct{ item node }

// cmpNode is `field <op> value(s)` or a `field eq/ne null` check.
type cmpNode struct {
	field  string
	op     Op
	vals   []token
	isNull bool
	nullNe bool // true for `ne null` (IS NOT NULL)
}

// funcNode is `contains|startswith|endswith(field, 'value')`.
type funcNode struct {
	name  Op
	field string
	val   token
}

// --- parser ---

type parser struct {
	toks []token
	i    int
}

func (p *parser) cur() token  { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }
func (p *parser) isKw(v string) bool {
	t := p.cur()
	return t.kind == tIdent && strings.EqualFold(t.val, v)
}

func (p *parser) parse() (node, error) {
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.cur().kind != tEOF {
		return nil, filterErr("unexpected %q at position %d in $filter", p.cur().val, p.cur().pos)
	}
	return n, nil
}

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	items := []node{left}
	for p.isKw("or") {
		p.next()
		r, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if len(items) == 1 {
		return left, nil
	}
	return &boolNode{op: "or", items: items}, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	items := []node{left}
	for p.isKw("and") {
		p.next()
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if len(items) == 1 {
		return left, nil
	}
	return &boolNode{op: "and", items: items}, nil
}

func (p *parser) parseUnary() (node, error) {
	if p.isKw("not") {
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &notNode{item: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (node, error) {
	t := p.cur()
	if t.kind == tLParen {
		p.next()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.cur().kind != tRParen {
			return nil, filterErr("unbalanced parentheses in $filter (expected ')' at position %d)", p.cur().pos)
		}
		p.next()
		return inner, nil
	}
	// A function call is `name(` where name is a known string function.
	if t.kind == tIdent && isFunc(t.val) && p.toks[p.i+1].kind == tLParen {
		return p.parseFunc()
	}
	return p.parseComparison()
}

func isFunc(v string) bool {
	switch strings.ToLower(v) {
	case "contains", "startswith", "endswith":
		return true
	}
	return false
}

func (p *parser) parseFunc() (node, error) {
	name := Op(strings.ToLower(p.next().val))
	p.next() // '('
	if p.cur().kind != tIdent {
		return nil, filterErr("%s(...) expects a field name at position %d in $filter", name, p.cur().pos)
	}
	field := p.next().val
	if p.cur().kind != tComma {
		return nil, filterErr("%s(%s, ...) expects a comma then a value at position %d in $filter", name, field, p.cur().pos)
	}
	p.next() // ','
	val := p.cur()
	if val.kind != tStr {
		return nil, filterErr("%s(%s, ...) expects a quoted string value at position %d in $filter", name, field, val.pos)
	}
	p.next()
	if p.cur().kind != tRParen {
		return nil, filterErr("unbalanced parentheses in %s(...) (expected ')' at position %d)", name, p.cur().pos)
	}
	p.next()
	return &funcNode{name: name, field: field, val: val}, nil
}

func (p *parser) parseComparison() (node, error) {
	t := p.cur()
	if t.kind != tIdent {
		return nil, filterErr("expected a field name at position %d in $filter, found %q", t.pos, t.val)
	}
	field := p.next().val

	// Operator: a keyword, or the two-token `not in`.
	opTok := p.cur()
	if opTok.kind != tIdent {
		return nil, filterErr("expected an operator after %q at position %d in $filter", field, opTok.pos)
	}
	op := Op(strings.ToLower(opTok.val))
	p.next()
	if op == "not" {
		if !p.isKw("in") {
			return nil, filterErr("expected 'in' after 'not' at position %d in $filter", p.cur().pos)
		}
		p.next()
		op = OpNotIn
	}

	switch op {
	case OpEq, OpNe, OpLt, OpLe, OpGt, OpGe:
		if op == OpEq || op == OpNe {
			if p.isKw("null") {
				p.next()
				return &cmpNode{field: field, isNull: true, nullNe: op == OpNe}, nil
			}
		}
		v := p.cur()
		if !isValueTok(v) {
			return nil, filterErr("expected a value after '%s' at position %d in $filter", op, v.pos)
		}
		p.next()
		return &cmpNode{field: field, op: op, vals: []token{v}}, nil
	case OpIn, OpNotIn:
		vals, err := p.parseValueList(field, op)
		if err != nil {
			return nil, err
		}
		return &cmpNode{field: field, op: op, vals: vals}, nil
	default:
		return nil, filterErr("unknown operator %q at position %d in $filter", opTok.val, opTok.pos)
	}
}

func (p *parser) parseValueList(field string, op Op) ([]token, error) {
	if p.cur().kind != tLParen {
		return nil, filterErr("'%s %s' expects a parenthesized list at position %d in $filter", field, op, p.cur().pos)
	}
	p.next()
	var vals []token
	for {
		v := p.cur()
		if !isValueTok(v) {
			return nil, filterErr("expected a value in '%s %s (...)' at position %d in $filter", field, op, v.pos)
		}
		vals = append(vals, v)
		p.next()
		if p.cur().kind == tComma {
			p.next()
			continue
		}
		break
	}
	if p.cur().kind != tRParen {
		return nil, filterErr("unbalanced parentheses in '%s %s (...)' (expected ')' at position %d)", field, op, p.cur().pos)
	}
	p.next()
	if len(vals) == 0 {
		return nil, filterErr("'%s %s ()' needs at least one value", field, op)
	}
	return vals, nil
}

// isValueTok reports whether a token can be a comparison operand: a string, a
// number/datetime, or a true/false literal (null is handled separately).
func isValueTok(t token) bool {
	if t.kind == tStr || t.kind == tNum {
		return true
	}
	return t.kind == tIdent && (strings.EqualFold(t.val, "true") || strings.EqualFold(t.val, "false"))
}

// --- binder: AST -> SQL, with per-field validation ---

type binder struct {
	fs    *FieldSet
	param int
	args  []any
}

func (b *binder) placeholder(v any) string {
	b.args = append(b.args, v)
	s := "$" + strconv.Itoa(b.param)
	b.param++
	return s
}

func (b *binder) bind(n node) (string, error) {
	switch t := n.(type) {
	case *boolNode:
		parts := make([]string, 0, len(t.items))
		for _, it := range t.items {
			s, err := b.bind(it)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		sep := " AND "
		if t.op == "or" {
			sep = " OR "
		}
		return "(" + strings.Join(parts, sep) + ")", nil
	case *notNode:
		s, err := b.bind(t.item)
		if err != nil {
			return "", err
		}
		return "NOT " + s, nil
	case *cmpNode:
		return b.bindCmp(t)
	case *funcNode:
		return b.bindFunc(t)
	default:
		return "", fmt.Errorf("query: unhandled filter node %T", n)
	}
}

func (b *binder) field(name string) (Field, error) {
	f, ok := b.fs.lookup(name)
	if ok {
		return f, nil
	}
	if s := b.fs.suggest(name); s != "" {
		return Field{}, filterErr("unknown field %q in $filter; did you mean %q? Known fields: %s", name, s, b.fs.known())
	}
	return Field{}, filterErr("unknown field %q in $filter. Known fields: %s", name, b.fs.known())
}

func (b *binder) bindCmp(c *cmpNode) (string, error) {
	f, err := b.field(c.field)
	if err != nil {
		return "", err
	}
	if c.isNull { // `field eq/ne null` — a null check is universal
		if c.nullNe {
			return f.Column + " IS NOT NULL", nil
		}
		return f.Column + " IS NULL", nil
	}
	if !f.allows(c.op) {
		return "", filterErr("operator %q is not allowed on field %q; allowed: %s", c.op, f.Name, opList(f.Ops))
	}
	switch c.op {
	case OpIn, OpNotIn:
		ph := make([]string, 0, len(c.vals))
		for _, v := range c.vals {
			arg, err := b.value(f, v)
			if err != nil {
				return "", err
			}
			ph = append(ph, b.placeholder(arg))
		}
		kw := "IN"
		if c.op == OpNotIn {
			kw = "NOT IN"
		}
		return f.Column + " " + kw + " (" + strings.Join(ph, ", ") + ")", nil
	default:
		arg, err := b.value(f, c.vals[0])
		if err != nil {
			return "", err
		}
		return f.Column + " " + sqlCmp(c.op) + " " + b.placeholder(arg), nil
	}
}

func (b *binder) bindFunc(fn *funcNode) (string, error) {
	f, err := b.field(fn.field)
	if err != nil {
		return "", err
	}
	if !f.allows(fn.name) {
		return "", filterErr("operator %q is not allowed on field %q; allowed: %s", fn.name, f.Name, opList(f.Ops))
	}
	if f.Kind == KindJSON { // contains on JSON is containment, not substring
		return f.Column + " @> " + b.placeholder(fn.val.val) + "::jsonb", nil
	}
	// String match: case-insensitive, with the operand's LIKE metacharacters
	// escaped so they match literally.
	esc := escapeLike(fn.val.val)
	var pat string
	switch fn.name {
	case OpStartsWith:
		pat = esc + "%"
	case OpEndsWith:
		pat = "%" + esc
	default: // contains
		pat = "%" + esc + "%"
	}
	return f.Column + " ILIKE " + b.placeholder(pat) + " ESCAPE '\\'", nil
}

func sqlCmp(op Op) string {
	switch op {
	case OpEq:
		return "="
	case OpNe:
		return "<>"
	case OpLt:
		return "<"
	case OpLe:
		return "<="
	case OpGt:
		return ">"
	case OpGe:
		return ">="
	}
	return "="
}

// value type-checks a token against the field's kind and enum, returning the
// Go value to bind. Messages name the field and what it expected.
func (b *binder) value(f Field, t token) (any, error) {
	// true/false literals arrive as idents.
	if t.kind == tIdent {
		if f.Kind == KindBool {
			return strings.EqualFold(t.val, "true"), nil
		}
		return nil, filterErr("field %q expects %s, got %q", f.Name, kindDesc(f.Kind), t.val)
	}
	switch f.Kind {
	case KindText, KindInet:
		if t.kind != tStr {
			return nil, filterErr("field %q expects a quoted string, got %q", f.Name, t.val)
		}
		if err := b.checkEnum(f, t.val); err != nil {
			return nil, err
		}
		return t.val, nil
	case KindUUID:
		if t.kind != tStr {
			return nil, filterErr("field %q expects a quoted uuid, got %q", f.Name, t.val)
		}
		if _, err := uuid.Parse(t.val); err != nil {
			return nil, filterErr("field %q expects a uuid; %q is not a valid uuid", f.Name, t.val)
		}
		if err := b.checkEnum(f, t.val); err != nil {
			return nil, err
		}
		return t.val, nil
	case KindInt:
		if t.kind != tNum {
			return nil, filterErr("field %q expects an integer, got %q", f.Name, t.val)
		}
		n, err := strconv.ParseInt(t.val, 10, 64)
		if err != nil {
			return nil, filterErr("field %q expects an integer; %q is not one", f.Name, t.val)
		}
		return n, nil
	case KindTime:
		ts, err := parseTime(t.val)
		if err != nil {
			return nil, filterErr("field %q expects an RFC 3339 date-time (e.g. 2026-01-01T00:00:00Z), got %q", f.Name, t.val)
		}
		return ts, nil
	case KindBool:
		return nil, filterErr("field %q expects true or false, got %q", f.Name, t.val)
	default:
		return nil, filterErr("field %q is not comparable", f.Name)
	}
}

func (b *binder) checkEnum(f Field, v string) error {
	if len(f.Enum) == 0 {
		return nil
	}
	for _, e := range f.Enum {
		if e == v {
			return nil
		}
	}
	return filterErr("field %q only accepts %s; %q is not one", f.Name, opJoin(f.Enum), v)
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad time")
}

// escapeLike neutralizes LIKE metacharacters so a substring match is literal.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func kindDesc(k Kind) string {
	switch k {
	case KindText, KindInet:
		return "a quoted string"
	case KindUUID:
		return "a quoted uuid"
	case KindInt:
		return "an integer"
	case KindTime:
		return "a date-time"
	case KindBool:
		return "true or false"
	case KindJSON:
		return "a json value"
	}
	return "a value"
}

func opList(ops []Op) string {
	ss := make([]string, len(ops))
	for i, o := range ops {
		ss[i] = string(o)
	}
	return strings.Join(ss, ", ")
}

func opJoin(vs []string) string { return strings.Join(vs, ", ") }
