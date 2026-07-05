package loot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Pickit evaluates the Phase-5.3 NIP subset against generated [world.Item] values.
type Pickit struct {
	rules []pickitRule
}

// PickitResult describes the first Pickit rule matched by an item.
type PickitResult struct {
	Matched   bool
	RuleIndex int
	Line      int
	Rule      string
}

type pickitRule struct {
	line int
	text string
	expr pickitExpr
}

// LoadPickit reads and parses a Pickit file for later read-only item evaluation.
func LoadPickit(path string) (*Pickit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pickit %q: %w", path, err)
	}
	p, err := parsePickit(path, string(data))
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Evaluate returns the first configured rule matched by item, if any.
func (p *Pickit) Evaluate(item world.Item) PickitResult {
	if p == nil {
		return PickitResult{}
	}
	for idx, rule := range p.rules {
		if rule.expr.eval(item) {
			return PickitResult{
				Matched:   true,
				RuleIndex: idx,
				Line:      rule.line,
				Rule:      rule.text,
			}
		}
	}
	return PickitResult{}
}

func parsePickit(path, content string) (*Pickit, error) {
	lines := strings.Split(content, "\n")
	rules := make([]pickitRule, 0)
	for idx, raw := range lines {
		lineNo := idx + 1
		text := normalizePickitLine(raw)
		if text == "" {
			continue
		}
		expr, err := parsePickitExpression(path, lineNo, text)
		if err != nil {
			return nil, err
		}
		rules = append(rules, pickitRule{line: lineNo, text: text, expr: expr})
	}
	return &Pickit{rules: rules}, nil
}

func normalizePickitLine(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" || strings.HasPrefix(text, ";") || strings.HasPrefix(text, "//") {
		return ""
	}
	if idx := strings.Index(text, "//"); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return text
}

func parsePickitExpression(path string, line int, text string) (pickitExpr, error) {
	tokens, err := lexPickit(path, line, text)
	if err != nil {
		return nil, err
	}
	parser := pickitParser{path: path, line: line, tokens: tokens}
	expr, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != tokenEOF {
		return nil, parser.errf("unexpected token %q", parser.peek().value)
	}
	return expr, nil
}

type pickitExpr interface {
	eval(world.Item) bool
}

type binaryExpr struct {
	op          tokenKind
	left, right pickitExpr
}

func (e binaryExpr) eval(item world.Item) bool {
	switch e.op {
	case tokenAnd:
		return e.left.eval(item) && e.right.eval(item)
	case tokenOr:
		return e.left.eval(item) || e.right.eval(item)
	default:
		return false
	}
}

type compareExpr struct {
	field pickitField
	op    tokenKind
	lit   pickitLiteral
}

func (e compareExpr) eval(item world.Item) bool {
	switch e.field.kind {
	case fieldName:
		return compareString(strings.ToLower(item.Code), e.op, e.lit.text)
	case fieldType:
		return compareString(strings.ToLower(item.Type), e.op, e.lit.text)
	case fieldQuality:
		return compareString(strings.ToLower(item.Quality.String()), e.op, e.lit.text)
	case fieldFlag:
		var has bool
		switch e.lit.text {
		case "identified":
			has = item.Identified
		case "ethereal":
			has = item.Ethereal
		default:
			return false
		}
		if e.op == tokenEqual {
			return has
		}
		return !has
	case fieldStat:
		for _, stat := range item.Stats {
			if stat.ID == e.field.statID && compareInt(int(stat.Value), e.op, e.lit.num) {
				return true
			}
		}
	}
	return false
}

func compareString(left string, op tokenKind, right string) bool {
	switch op {
	case tokenEqual:
		return left == right
	case tokenNotEqual:
		return left != right
	default:
		return false
	}
}

func compareInt(left int, op tokenKind, right int) bool {
	switch op {
	case tokenGreater:
		return left > right
	case tokenGreaterEqual:
		return left >= right
	case tokenLess:
		return left < right
	case tokenLessEqual:
		return left <= right
	case tokenEqual:
		return left == right
	case tokenNotEqual:
		return left != right
	default:
		return false
	}
}

type pickitFieldKind int

const (
	fieldName pickitFieldKind = iota
	fieldType
	fieldQuality
	fieldFlag
	fieldStat
)

type pickitField struct {
	kind   pickitFieldKind
	statID uint16
	label  string
}

type literalKind int

const (
	literalString literalKind = iota
	literalInt
)

type pickitLiteral struct {
	kind literalKind
	text string
	num  int
}

type pickitParser struct {
	path   string
	line   int
	tokens []pickitToken
	pos    int
}

func (p *pickitParser) parseOr() (pickitExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.match(tokenOr) {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{op: tokenOr, left: left, right: right}
	}
	return left, nil
}

func (p *pickitParser) parseAnd() (pickitExpr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.match(tokenAnd) {
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{op: tokenAnd, left: left, right: right}
	}
	return left, nil
}

func (p *pickitParser) parseComparison() (pickitExpr, error) {
	if p.match(tokenLParen) {
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match(tokenRParen) {
			return nil, p.errf("expected )")
		}
		return expr, nil
	}
	fieldTok := p.advance()
	if fieldTok.kind != tokenField {
		return nil, p.errf("expected field, got %q", fieldTok.value)
	}
	field, err := parsePickitField(fieldTok.value)
	if err != nil {
		return nil, p.errf("%v", err)
	}
	op := p.advance()
	if !isComparisonOperator(op.kind) {
		return nil, p.errf("expected comparison operator after [%s]", field.label)
	}
	litTok := p.advance()
	if litTok.kind != tokenIdentifier && litTok.kind != tokenString && litTok.kind != tokenInteger {
		return nil, p.errf("expected literal after operator")
	}
	lit, err := parsePickitLiteral(field, op.kind, litTok)
	if err != nil {
		return nil, p.errf("%v", err)
	}
	return compareExpr{field: field, op: op.kind, lit: lit}, nil
}

func (p *pickitParser) match(kind tokenKind) bool {
	if p.peek().kind != kind {
		return false
	}
	p.pos++
	return true
}

func (p *pickitParser) advance() pickitToken {
	tok := p.peek()
	if tok.kind != tokenEOF {
		p.pos++
	}
	return tok
}

func (p *pickitParser) peek() pickitToken {
	if p.pos >= len(p.tokens) {
		return pickitToken{kind: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *pickitParser) errf(format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", p.path, p.line, fmt.Sprintf(format, args...))
}

func parsePickitField(raw string) (pickitField, error) {
	label := strings.ToLower(strings.TrimSpace(raw))
	switch label {
	case "name":
		return pickitField{kind: fieldName, label: label}, nil
	case "type":
		return pickitField{kind: fieldType, label: label}, nil
	case "quality":
		return pickitField{kind: fieldQuality, label: label}, nil
	case "flag":
		return pickitField{kind: fieldFlag, label: label}, nil
	default:
		if strings.HasPrefix(label, "stat:") {
			id, err := strconv.ParseUint(strings.TrimPrefix(label, "stat:"), 10, 16)
			if err != nil {
				return pickitField{}, fmt.Errorf("invalid stat field [%s]", raw)
			}
			return pickitField{kind: fieldStat, statID: uint16(id), label: label}, nil
		}
		return pickitField{}, fmt.Errorf("unsupported keyword [%s]", raw)
	}
}

func parsePickitLiteral(field pickitField, op tokenKind, tok pickitToken) (pickitLiteral, error) {
	switch field.kind {
	case fieldName, fieldType, fieldQuality:
		if op != tokenEqual && op != tokenNotEqual {
			return pickitLiteral{}, fmt.Errorf("[%s] supports only == and !=", field.label)
		}
		if tok.kind == tokenInteger {
			return pickitLiteral{}, fmt.Errorf("[%s] requires a string literal", field.label)
		}
		return pickitLiteral{kind: literalString, text: strings.ToLower(tok.value)}, nil
	case fieldFlag:
		if op != tokenEqual && op != tokenNotEqual {
			return pickitLiteral{}, fmt.Errorf("[flag] supports only == and !=")
		}
		if tok.kind == tokenInteger {
			return pickitLiteral{}, fmt.Errorf("[flag] requires identified or ethereal")
		}
		value := strings.ToLower(tok.value)
		if value != "identified" && value != "ethereal" {
			return pickitLiteral{}, fmt.Errorf("unsupported flag %q", tok.value)
		}
		return pickitLiteral{kind: literalString, text: value}, nil
	case fieldStat:
		if tok.kind != tokenInteger {
			return pickitLiteral{}, fmt.Errorf("[%s] requires an integer literal", field.label)
		}
		n, err := strconv.Atoi(tok.value)
		if err != nil {
			return pickitLiteral{}, fmt.Errorf("invalid integer %q", tok.value)
		}
		return pickitLiteral{kind: literalInt, num: n}, nil
	default:
		return pickitLiteral{}, fmt.Errorf("unsupported field")
	}
}

func isComparisonOperator(kind tokenKind) bool {
	switch kind {
	case tokenGreater, tokenGreaterEqual, tokenLess, tokenLessEqual, tokenEqual, tokenNotEqual:
		return true
	default:
		return false
	}
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenField
	tokenIdentifier
	tokenString
	tokenInteger
	tokenAnd
	tokenOr
	tokenGreater
	tokenGreaterEqual
	tokenLess
	tokenLessEqual
	tokenEqual
	tokenNotEqual
	tokenLParen
	tokenRParen
)

type pickitToken struct {
	kind  tokenKind
	value string
}

func lexPickit(path string, line int, text string) ([]pickitToken, error) {
	tokens := make([]pickitToken, 0)
	for i := 0; i < len(text); {
		r := rune(text[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		switch {
		case strings.HasPrefix(text[i:], "&&"):
			tokens = append(tokens, pickitToken{kind: tokenAnd, value: "&&"})
			i += 2
		case strings.HasPrefix(text[i:], "||"):
			tokens = append(tokens, pickitToken{kind: tokenOr, value: "||"})
			i += 2
		case strings.HasPrefix(text[i:], ">="):
			tokens = append(tokens, pickitToken{kind: tokenGreaterEqual, value: ">="})
			i += 2
		case strings.HasPrefix(text[i:], "<="):
			tokens = append(tokens, pickitToken{kind: tokenLessEqual, value: "<="})
			i += 2
		case strings.HasPrefix(text[i:], "=="):
			tokens = append(tokens, pickitToken{kind: tokenEqual, value: "=="})
			i += 2
		case strings.HasPrefix(text[i:], "!="):
			tokens = append(tokens, pickitToken{kind: tokenNotEqual, value: "!="})
			i += 2
		case text[i] == '#':
			tokens = append(tokens, pickitToken{kind: tokenAnd, value: "#"})
			i++
		case text[i] == '>':
			tokens = append(tokens, pickitToken{kind: tokenGreater, value: ">"})
			i++
		case text[i] == '<':
			tokens = append(tokens, pickitToken{kind: tokenLess, value: "<"})
			i++
		case text[i] == '(':
			tokens = append(tokens, pickitToken{kind: tokenLParen, value: "("})
			i++
		case text[i] == ')':
			tokens = append(tokens, pickitToken{kind: tokenRParen, value: ")"})
			i++
		case text[i] == '[':
			end := strings.IndexByte(text[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("%s:%d: unterminated field", path, line)
			}
			value := text[i+1 : i+end]
			tokens = append(tokens, pickitToken{kind: tokenField, value: value})
			i += end + 1
		case text[i] == '"' || text[i] == '\'':
			quote := text[i]
			start := i + 1
			i++
			for i < len(text) && text[i] != quote {
				i++
			}
			if i >= len(text) {
				return nil, fmt.Errorf("%s:%d: unterminated string literal", path, line)
			}
			tokens = append(tokens, pickitToken{kind: tokenString, value: text[start:i]})
			i++
		case text[i] == '-' || isASCIIDigit(text[i]):
			start := i
			if text[i] == '-' {
				i++
			}
			if i >= len(text) || !isASCIIDigit(text[i]) {
				return nil, fmt.Errorf("%s:%d: invalid integer literal", path, line)
			}
			for i < len(text) && isASCIIDigit(text[i]) {
				i++
			}
			tokens = append(tokens, pickitToken{kind: tokenInteger, value: text[start:i]})
		case isIdentifierStart(text[i]):
			start := i
			i++
			for i < len(text) && isIdentifierPart(text[i]) {
				i++
			}
			tokens = append(tokens, pickitToken{kind: tokenIdentifier, value: text[start:i]})
		default:
			return nil, fmt.Errorf("%s:%d: unexpected character %q", path, line, text[i])
		}
	}
	tokens = append(tokens, pickitToken{kind: tokenEOF})
	return tokens, nil
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isIdentifierStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isIdentifierPart(b byte) bool {
	return isIdentifierStart(b) || isASCIIDigit(b)
}
