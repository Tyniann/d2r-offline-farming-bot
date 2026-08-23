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
	Matched            bool
	RuleIndex          int
	Line               int
	Rule               string
	RuleID             string
	ProfileID          string
	Action             Action
	ProfileRevision    uint64
	AssignmentRevision uint64
	Trace              []PickitTraceEntry
}

// PickitRuleSpec beschreibt eine geordnete Runtime-Regel samt stabiler Herkunft.
type PickitRuleSpec struct {
	ProfileID          string
	RuleID             string
	Action             Action
	Expression         string
	ProfileRevision    uint64
	AssignmentRevision uint64
}

// PickitTraceEntry beschreibt genau eine tatsächlich ausgewertete Regel.
type PickitTraceEntry struct {
	RuleIndex          int
	ProfileID          string
	RuleID             string
	Action             Action
	Expression         string
	Matched            bool
	ProfileRevision    uint64
	AssignmentRevision uint64
}

type pickitRule struct {
	line               int
	text               string
	profileID          string
	ruleID             string
	action             Action
	profileRevision    uint64
	assignmentRevision uint64
	expr               pickitExpr
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
	trace := make([]PickitTraceEntry, 0, len(p.rules))
	for idx, rule := range p.rules {
		matched := rule.expr.eval(item)
		trace = append(trace, PickitTraceEntry{
			RuleIndex:          idx,
			ProfileID:          rule.profileID,
			RuleID:             rule.ruleID,
			Action:             rule.action,
			Expression:         rule.text,
			Matched:            matched,
			ProfileRevision:    rule.profileRevision,
			AssignmentRevision: rule.assignmentRevision,
		})
		if matched {
			return PickitResult{
				Matched:            true,
				RuleIndex:          idx,
				Line:               rule.line,
				Rule:               rule.text,
				RuleID:             rule.ruleID,
				ProfileID:          rule.profileID,
				Action:             rule.action,
				ProfileRevision:    rule.profileRevision,
				AssignmentRevision: rule.assignmentRevision,
				Trace:              trace,
			}
		}
	}
	return PickitResult{Trace: trace}
}

// CompilePickitRules validiert und kompiliert geordnete Profilregeln für die bestehende First-Match-Auswertung.
func CompilePickitRules(source string, specs []PickitRuleSpec) (*Pickit, error) {
	rules := make([]pickitRule, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for index, spec := range specs {
		if strings.TrimSpace(spec.ProfileID) == "" || strings.TrimSpace(spec.RuleID) == "" {
			return nil, fmt.Errorf("%s:%d: profile and rule id are required", source, index+1)
		}
		if !spec.Action.Valid() {
			return nil, fmt.Errorf("%s:%d: unsupported action %q", source, index+1, spec.Action)
		}
		key := spec.ProfileID + "\x00" + spec.RuleID
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%s:%d: duplicate rule %s/%s", source, index+1, spec.ProfileID, spec.RuleID)
		}
		seen[key] = struct{}{}
		canonical, expr, err := canonicalPickitExpression(source, index+1, spec.Expression)
		if err != nil {
			return nil, err
		}
		rules = append(rules, pickitRule{
			line: index + 1, text: canonical, profileID: spec.ProfileID,
			ruleID: spec.RuleID, action: spec.Action, expr: expr,
			profileRevision: spec.ProfileRevision, assignmentRevision: spec.AssignmentRevision,
		})
	}
	return &Pickit{rules: rules}, nil
}

// CanonicalPickitExpression validiert einen Ausdruck und serialisiert ihn eindeutig.
func CanonicalPickitExpression(expression string) (string, error) {
	canonical, _, err := canonicalPickitExpression("expression", 1, expression)
	return canonical, err
}

// PickitRuleSummary ist ein sprachneutraler Darstellungshinweis aus dem geparsten Ausdrucksbaum.
type PickitRuleSummary struct {
	Kind   string
	Params PickitRuleSummaryParams
}

// PickitRuleSummaryParams enthält ausschließlich stabile Katalogschlüssel und primitive Filterwerte.
// Leere Slices und nil-Zeiger bedeuten, dass der jeweilige Filter nicht Teil der Summary ist.
type PickitRuleSummaryParams struct {
	Codes          []string
	Types          []string
	Qualities      []string
	Tiers          []string
	SetKey         string
	UniqueKey      string
	SocketOperator string
	SocketCount    *int
	Ethereal       *bool
}

// SummarizePickitExpression parst einen Ausdruck mit dem Runtime-Parser und liefert eine sichere UI-Summary.
func SummarizePickitExpression(expression string) (PickitRuleSummary, error) {
	_, expr, err := canonicalPickitExpression("expression", 1, expression)
	if err != nil {
		return PickitRuleSummary{}, err
	}
	groups, ok := summarizePickitGroups(expr)
	if !ok {
		return customPickitRuleSummary(), nil
	}
	params, ok := summarizePickitParams(groups)
	if !ok {
		return customPickitRuleSummary(), nil
	}
	if sockets, ok := groups["sockets"]; ok {
		if len(sockets.values) != 1 {
			return customPickitRuleSummary(), nil
		}
		count, parseErr := strconv.Atoi(sockets.values[0])
		if parseErr != nil {
			return customPickitRuleSummary(), nil
		}
		params.SocketOperator = sockets.operator
		params.SocketCount = &count
		return PickitRuleSummary{Kind: "socket_filter", Params: params}, nil
	}
	if len(groups) == 2 {
		if _, quality := groups["quality"]; quality {
			if _, tier := groups["tier"]; tier {
				return PickitRuleSummary{Kind: "quality_tier", Params: params}, nil
			}
		}
	}
	if len(groups) != 1 {
		return customPickitRuleSummary(), nil
	}
	for field, group := range groups {
		if group.operator != "==" {
			return customPickitRuleSummary(), nil
		}
		if field == "type" && len(group.values) == 1 {
			switch group.values[0] {
			case "rune":
				return PickitRuleSummary{Kind: "runes"}, nil
			case "rpot":
				return PickitRuleSummary{Kind: "rejuvenation"}, nil
			}
		}
		kinds := map[string]string{
			"name": "item_codes", "type": "item_types", "quality": "quality",
			"tier": "tier", "setitem": "set_item", "uniqueitem": "unique_item",
		}
		if kind := kinds[field]; kind != "" {
			return PickitRuleSummary{Kind: kind, Params: params}, nil
		}
	}
	return customPickitRuleSummary(), nil
}

func customPickitRuleSummary() PickitRuleSummary {
	return PickitRuleSummary{Kind: "custom"}
}

func summarizePickitParams(groups map[string]pickitSummaryGroup) (PickitRuleSummaryParams, bool) {
	params := PickitRuleSummaryParams{}
	for field, group := range groups {
		if field != "sockets" && field != "flag" && group.operator != "==" {
			return PickitRuleSummaryParams{}, false
		}
		switch field {
		case "name":
			params.Codes = append([]string(nil), group.values...)
		case "type":
			params.Types = append([]string(nil), group.values...)
		case "quality":
			params.Qualities = append([]string(nil), group.values...)
		case "tier":
			params.Tiers = append([]string(nil), group.values...)
		case "setitem":
			if len(group.values) != 1 {
				return PickitRuleSummaryParams{}, false
			}
			params.SetKey = group.values[0]
		case "uniqueitem":
			if len(group.values) != 1 {
				return PickitRuleSummaryParams{}, false
			}
			params.UniqueKey = group.values[0]
		case "flag":
			if len(group.values) != 1 || group.values[0] != "ethereal" {
				return PickitRuleSummaryParams{}, false
			}
			value := group.operator == "=="
			params.Ethereal = &value
		case "sockets":
			// The caller validates and projects the numeric socket predicate.
		default:
			return PickitRuleSummaryParams{}, false
		}
	}
	return params, true
}

type pickitSummaryGroup struct {
	operator string
	values   []string
}

func summarizePickitGroups(expr pickitExpr) (map[string]pickitSummaryGroup, bool) {
	terms := flattenPickitBinary(expr, tokenAnd)
	result := make(map[string]pickitSummaryGroup, len(terms))
	for _, term := range terms {
		comparisons := flattenPickitBinary(term, tokenOr)
		var field string
		group := pickitSummaryGroup{}
		for _, comparison := range comparisons {
			value, ok := comparison.(compareExpr)
			if !ok {
				return nil, false
			}
			if field == "" {
				field = value.field.label
				group.operator = tokenText(value.op)
			}
			if field != value.field.label || group.operator != tokenText(value.op) {
				return nil, false
			}
			literal := value.lit.text
			if value.lit.kind == literalInt {
				literal = strconv.Itoa(value.lit.num)
			}
			group.values = append(group.values, literal)
		}
		if _, duplicate := result[field]; duplicate {
			return nil, false
		}
		result[field] = group
	}
	return result, true
}

func flattenPickitBinary(expr pickitExpr, operator tokenKind) []pickitExpr {
	value, ok := expr.(binaryExpr)
	if !ok || value.op != operator {
		return []pickitExpr{expr}
	}
	left := flattenPickitBinary(value.left, operator)
	return append(left, flattenPickitBinary(value.right, operator)...)
}

// RequiresIdentificationForKeep reports whether final keep/stash evaluation must wait for identification.
func RequiresIdentificationForKeep(item world.Item) bool {
	if item.Identified {
		return false
	}
	switch item.Quality {
	case world.ItemQualityMagic, world.ItemQualitySet, world.ItemQualityRare, world.ItemQualityUnique, world.ItemQualityCrafted:
		return true
	default:
		return false
	}
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
		rules = append(rules, pickitRule{
			line: lineNo, text: text, profileID: path, ruleID: fmt.Sprintf("line-%d", lineNo),
			action: ActionKeep, expr: expr,
		})
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

func canonicalPickitExpression(path string, line int, text string) (string, pickitExpr, error) {
	expr, err := parsePickitExpression(path, line, strings.TrimSpace(text))
	if err != nil {
		return "", nil, err
	}
	return formatPickitExpr(expr, 0), expr, nil
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
	case fieldTier:
		return compareString(strings.ToLower(item.BaseTier.String()), e.op, e.lit.text)
	case fieldSetItem:
		if !item.IdentityValid || item.IdentityKind != world.ItemIdentitySet {
			return false
		}
		return compareString(strings.ToLower(item.IdentityKey), e.op, strings.ToLower(e.lit.text))
	case fieldUniqueItem:
		if !item.IdentityValid || item.IdentityKind != world.ItemIdentityUnique {
			return false
		}
		return compareString(strings.ToLower(item.IdentityKey), e.op, strings.ToLower(e.lit.text))
	case fieldFlag:
		var has bool
		switch e.lit.text {
		case "identified":
			has = item.Identified
		case "ethereal":
			has = item.Ethereal
		case "socketed":
			// Socket predicates stay fail-closed when evidence is unavailable,
			// including != socketed — unknown is never treated as unsocketed.
			if !item.SocketsAvailable {
				return false
			}
			has = item.Socketed
		default:
			return false
		}
		if e.op == tokenEqual {
			return has
		}
		return !has
	case fieldSockets:
		if !item.SocketsAvailable {
			return false
		}
		return compareInt(item.Sockets, e.op, e.lit.num)
	case fieldStat:
		if !item.Identified {
			return false
		}
		for _, stat := range item.Stats {
			if stat.ID == e.field.statID && compareInt(int(stat.Value), e.op, e.lit.num) {
				return true
			}
		}
	}
	return false
}

func formatPickitExpr(expr pickitExpr, parentPrecedence int) string {
	switch value := expr.(type) {
	case compareExpr:
		literal := strconv.Itoa(value.lit.num)
		if value.lit.kind == literalString {
			literal = quotePickitString(value.lit.text)
		}
		return fmt.Sprintf("[%s] %s %s", value.field.label, tokenText(value.op), literal)
	case binaryExpr:
		precedence := 1
		if value.op == tokenAnd {
			precedence = 2
		}
		formatted := formatPickitExpr(value.left, precedence) + " " + tokenText(value.op) + " " + formatPickitExpr(value.right, precedence)
		if precedence < parentPrecedence {
			return "(" + formatted + ")"
		}
		return formatted
	default:
		return ""
	}
}

func quotePickitString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func tokenText(kind tokenKind) string {
	switch kind {
	case tokenAnd:
		return "&&"
	case tokenOr:
		return "||"
	case tokenGreater:
		return ">"
	case tokenGreaterEqual:
		return ">="
	case tokenLess:
		return "<"
	case tokenLessEqual:
		return "<="
	case tokenEqual:
		return "=="
	case tokenNotEqual:
		return "!="
	default:
		return ""
	}
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
	fieldTier
	fieldSetItem
	fieldUniqueItem
	fieldFlag
	fieldStat
	fieldSockets
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
	case "tier":
		return pickitField{kind: fieldTier, label: label}, nil
	case "setitem":
		return pickitField{kind: fieldSetItem, label: label}, nil
	case "uniqueitem":
		return pickitField{kind: fieldUniqueItem, label: label}, nil
	case "flag":
		return pickitField{kind: fieldFlag, label: label}, nil
	case "sockets":
		// [sockets] is a dedicated field (Gate 19.0), not an alias for [stat:194],
		// so white/gray bases can match without the Identify gate used by raw stats.
		return pickitField{kind: fieldSockets, label: label}, nil
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
	case fieldName, fieldType, fieldQuality, fieldTier, fieldSetItem, fieldUniqueItem:
		if op != tokenEqual && op != tokenNotEqual {
			return pickitLiteral{}, fmt.Errorf("[%s] supports only == and !=", field.label)
		}
		if tok.kind == tokenInteger {
			return pickitLiteral{}, fmt.Errorf("[%s] requires a string literal", field.label)
		}
		value := strings.ToLower(tok.value)
		if field.kind == fieldTier && value != string(world.BaseTierUnknown) && value != string(world.BaseTierNormal) && value != string(world.BaseTierExceptional) && value != string(world.BaseTierElite) {
			return pickitLiteral{}, fmt.Errorf("unsupported tier %q", tok.value)
		}
		if field.kind == fieldSetItem || field.kind == fieldUniqueItem {
			kind := world.ItemIdentitySet
			if field.kind == fieldUniqueItem {
				kind = world.ItemIdentityUnique
			}
			entry, ok := world.LookupItemIdentityKey(kind, tok.value)
			if !ok {
				return pickitLiteral{}, fmt.Errorf("unknown [%s] reference %q", field.label, tok.value)
			}
			value = entry.Key
		}
		return pickitLiteral{kind: literalString, text: value}, nil
	case fieldFlag:
		if op != tokenEqual && op != tokenNotEqual {
			return pickitLiteral{}, fmt.Errorf("[flag] supports only == and !=")
		}
		if tok.kind == tokenInteger {
			return pickitLiteral{}, fmt.Errorf("[flag] requires identified, ethereal, or socketed")
		}
		value := strings.ToLower(tok.value)
		if value != "identified" && value != "ethereal" && value != "socketed" {
			return pickitLiteral{}, fmt.Errorf("unsupported flag %q", tok.value)
		}
		return pickitLiteral{kind: literalString, text: value}, nil
	case fieldSockets, fieldStat:
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
			i++
			var value strings.Builder
			for i < len(text) && text[i] != quote {
				if text[i] == '\\' {
					if i+1 < len(text) && (text[i+1] == quote || text[i+1] == '\\') {
						i++
					} else {
						// Andere Backslash-Folgen bleiben wie im bisherigen Parser literal;
						// nur Quote und Backslash besitzen eine Escape-Bedeutung.
						value.WriteByte('\\')
						i++
						continue
					}
				}
				value.WriteByte(text[i])
				i++
			}
			if i >= len(text) {
				return nil, fmt.Errorf("%s:%d: unterminated string literal", path, line)
			}
			tokens = append(tokens, pickitToken{kind: tokenString, value: value.String()})
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
