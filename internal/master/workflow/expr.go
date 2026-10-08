package workflow

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// Templates put data into the settings of steps: {{trigger.server.name}} is the name of the
// server of the trigger, {{steps.count.value | add:1}} a value of an earlier step plus one. A
// template that is a single {{…}} keeps the type of its value, e.g. a list for a loop; others
// become text. Data are what JSON has: text, numbers, true and false, lists and objects.

const (
	maxTemplate = 4000
	maxPattern  = 500
	// maxText is the longest text a template renders, so that loops can't grow it without end.
	maxText = 64 << 10
)

var (
	pathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_-]+)*$`)
	// filters and how many arguments each takes at most.
	filters = map[string]int{
		"upper": 0, "lower": 0, "trim": 0, "length": 0, "json": 0, "first": 0, "last": 0, "number": 0,
		"default": 1, "join": 1, "split": 1, "round": 1, "add": 1, "sub": 1, "mul": 1, "div": 1, "map": 1, "replace": 2,
	}
	parsed sync.Map // templates by their text, as steps render the same ones again and again
	cached atomic.Int32
)

// maxCached are the templates kept parsed, far more than workflows have, so that changed ones
// can't fill the memory.
const maxCached = 50_000

// tmpl is a parsed template: texts and expressions.
type tmpl []part

type part struct {
	text string
	expr *expr
}

// expr reads a value by its path and passes it through filters, e.g. servers | map:name.
type expr struct {
	path    []string
	filters []call
}

type call struct {
	name string
	args []string
}

// parse reads a template, or returns why it is invalid.
func parse(s string) (tmpl, error) {
	if t, ok := parsed.Load(s); ok {
		return t.(tmpl), nil
	}
	if utf8.RuneCountInString(s) > maxTemplate {
		return nil, fmt.Errorf("it is longer than %d characters", maxTemplate)
	}
	var t tmpl
	for rest := s; rest != ""; {
		i := strings.Index(rest, "{{")
		if i < 0 {
			t = append(t, part{text: rest})
			break
		}
		j := strings.Index(rest[i:], "}}")
		if j < 0 {
			return nil, fmt.Errorf("a {{ has no }}")
		}
		if i > 0 {
			t = append(t, part{text: rest[:i]})
		}
		e, err := parseExpr(rest[i+2 : i+j])
		if err != nil {
			return nil, fmt.Errorf("{{%s}}: %w", rest[i+2:i+j], err)
		}
		t = append(t, part{expr: e})
		rest = rest[i+j+2:]
	}
	if cached.Add(1) <= maxCached {
		parsed.Store(s, t)
	}
	return t, nil
}

// parseExpr reads what is between {{ and }}: a path and filters, each after a |, with
// arguments after colons, quoted if they contain spaces, colons or bars.
func parseExpr(s string) (*expr, error) {
	segments, err := split(s, '|')
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(segments[0])
	if !pathPattern.MatchString(path) {
		return nil, fmt.Errorf("%q is no path like trigger.server.name", path)
	}
	e := &expr{path: strings.Split(path, ".")}
	for _, seg := range segments[1:] {
		args, err := split(strings.TrimSpace(seg), ':')
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(args[0])
		most, ok := filters[name]
		switch {
		case !ok:
			return nil, fmt.Errorf("there is no filter %q", name)
		case len(args)-1 > most:
			return nil, fmt.Errorf("the filter %s takes at most %d values", name, most)
		}
		c := call{name: name}
		for _, a := range args[1:] {
			c.args = append(c.args, unquote(strings.TrimSpace(a)))
		}
		e.filters = append(e.filters, c)
	}
	return e, nil
}

// split splits s at sep outside of quotes.
func split(s string, sep rune) ([]string, error) {
	var parts []string
	var quote rune
	start := 0
	for i, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
		case r == '"' || r == '\'':
			quote = r
		case r == sep:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("a quote isn't closed")
	}
	return append(parts, s[start:]), nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// lookup finds the value of a path in data.
type lookup func(path []string) any

// eval returns the value of a template: that of its only expression, or the text of all its
// parts.
func (t tmpl) eval(get lookup) (any, error) {
	if len(t) == 1 && t[0].expr != nil {
		return t[0].expr.eval(get)
	}
	var b strings.Builder
	for _, p := range t {
		if p.expr == nil {
			b.WriteString(p.text)
			continue
		}
		v, err := p.expr.eval(get)
		if err != nil {
			return nil, err
		}
		b.WriteString(text(v))
		if b.Len() > maxText {
			return nil, fmt.Errorf("the text is longer than %d KB", maxText>>10)
		}
	}
	return b.String(), nil
}

func (e *expr) eval(get lookup) (any, error) {
	v := get(e.path)
	for _, f := range e.filters {
		var err error
		if v, err = apply(f, v); err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return v, nil
}

// apply passes a value through a filter.
func apply(f call, v any) (any, error) {
	arg := func(i int, fallback string) string {
		if i < len(f.args) {
			return f.args[i]
		}
		return fallback
	}
	switch f.name {
	case "upper":
		return strings.ToUpper(text(v)), nil
	case "lower":
		return strings.ToLower(text(v)), nil
	case "trim":
		return strings.TrimSpace(text(v)), nil
	case "length":
		switch v := v.(type) {
		case []any:
			return float64(len(v)), nil
		case map[string]any:
			return float64(len(v)), nil
		}
		return float64(utf8.RuneCountInString(text(v))), nil
	case "json":
		b, err := json.Marshal(v)
		return string(b), err
	case "default":
		if empty(v) {
			return arg(0, ""), nil
		}
		return v, nil
	case "join":
		list := items(v)
		texts := make([]string, len(list))
		for i, item := range list {
			texts[i] = text(item)
		}
		return strings.Join(texts, arg(0, ", ")), nil
	case "split":
		list := []any{}
		for _, s := range strings.Split(text(v), arg(0, ",")) {
			if s = strings.TrimSpace(s); s != "" {
				list = append(list, s)
			}
		}
		return list, nil
	case "first", "last":
		list := items(v)
		switch {
		case len(list) == 0:
			return nil, nil
		case f.name == "first":
			return list[0], nil
		}
		return list[len(list)-1], nil
	case "map":
		list := items(v)
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = get(item, strings.Split(arg(0, ""), "."))
		}
		return out, nil
	case "replace":
		return strings.ReplaceAll(text(v), arg(0, ""), arg(1, "")), nil
	}
	n, ok := number(v)
	if !ok {
		return nil, fmt.Errorf("%q is no number", text(v))
	}
	var by float64
	if f.name != "number" && f.name != "round" || len(f.args) > 0 {
		if by, ok = number(arg(0, "")); !ok {
			return nil, fmt.Errorf("%q is no number", arg(0, ""))
		}
	}
	switch f.name {
	case "add":
		n += by
	case "sub":
		n -= by
	case "mul":
		n *= by
	case "div":
		if by == 0 {
			return nil, fmt.Errorf("it divides by 0")
		}
		n /= by
	case "round":
		p := math.Pow(10, math.Round(by))
		n = math.Round(n*p) / p
	}
	return n, nil
}

// get finds a path in a value: keys of objects, and indexes of lists from 0.
func get(v any, path []string) any {
	for _, key := range path {
		switch c := v.(type) {
		case map[string]any:
			v = c[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(c) {
				return nil
			}
			v = c[i]
		default:
			return nil
		}
	}
	return v
}

// text is a value as text: numbers without needless digits, lists and objects as JSON.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// number reads a number, also from text.
func number(v any) (float64, bool) {
	var n float64
	switch v := v.(type) {
	case float64:
		n = v
	case string:
		var err error
		if n, err = strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}

// items are the elements of a list, the values of an object, or the lines of a text, with
// commas separating them too, so that a loop can go through any of them.
func items(v any) []any {
	switch v := v.(type) {
	case nil:
		return nil
	case []any:
		return v
	case map[string]any:
		list := make([]any, 0, len(v))
		for _, k := range slices.Sorted(maps.Keys(v)) {
			list = append(list, v[k])
		}
		return list
	case string:
		var list []any
		for _, s := range strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == ',' }) {
			if s = strings.TrimSpace(s); s != "" {
				list = append(list, s)
			}
		}
		return list
	}
	return []any{v}
}

// empty tells whether a value is nothing: missing, empty text, an empty list or object, or false.
func empty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case bool:
		return !v
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

// Condition is true if all or any of its rules are.
type Condition struct {
	// Match is all or any.
	Match string `json:"match"`
	Rules []Rule `json:"rules"`
}

// Rule compares two templates, or is a group of rules.
type Rule struct {
	Left  string     `json:"left,omitempty"`
	Op    string     `json:"op,omitempty"`
	Right string     `json:"right,omitempty"`
	Group *Condition `json:"group,omitempty"`
}

const (
	maxRules      = 20
	maxGroupDepth = 3
)

// ops are the comparisons of rules, and whether they take a right side.
var ops = map[string]bool{
	"eq": true, "ne": true, "gt": true, "ge": true, "lt": true, "le": true, "contains": true, "notContains": true,
	"startsWith": true, "endsWith": true, "matches": true, "empty": false, "notEmpty": false,
}

func (c *Condition) check(depth int) error {
	switch {
	case c == nil:
		return fmt.Errorf("add a condition")
	case c.Match != "all" && c.Match != "any":
		return fmt.Errorf("choose whether all or any rules must apply")
	case len(c.Rules) == 0 || len(c.Rules) > maxRules:
		return fmt.Errorf("add 1 to %d rules", maxRules)
	case depth > maxGroupDepth:
		return fmt.Errorf("groups go at most %d deep", maxGroupDepth)
	}
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Group != nil {
			r.Left, r.Op, r.Right = "", "", ""
			if err := r.Group.check(depth + 1); err != nil {
				return err
			}
			continue
		}
		right, ok := ops[r.Op]
		if !ok {
			return fmt.Errorf("choose how to compare")
		}
		if !right {
			r.Right = ""
		}
		if r.Op == "matches" && !strings.Contains(r.Right, "{{") {
			if _, err := pattern(r.Right); err != nil {
				return err
			}
		}
	}
	return nil
}

// pattern compiles a regular expression of a rule. Go's expressions take linear time, so a
// pattern can't hold up a run.
func pattern(s string) (*regexp.Regexp, error) {
	if len(s) > maxPattern {
		return nil, fmt.Errorf("the pattern is longer than %d characters", maxPattern)
	}
	re, err := regexp.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("%q is no regular expression", s)
	}
	return re, nil
}

// eval tells whether a condition is true, with the values of its templates from value.
func (c *Condition) eval(value func(string) (any, error)) (bool, error) {
	for _, r := range c.Rules {
		ok, err := r.eval(value)
		switch {
		case err != nil:
			return false, err
		case ok && c.Match == "any":
			return true, nil
		case !ok && c.Match == "all":
			return false, nil
		}
	}
	return c.Match == "all", nil
}

func (r Rule) eval(value func(string) (any, error)) (bool, error) {
	if r.Group != nil {
		return r.Group.eval(value)
	}
	left, err := value(r.Left)
	if err != nil {
		return false, err
	}
	var right any
	if ops[r.Op] {
		if right, err = value(r.Right); err != nil {
			return false, err
		}
	}
	return compare(left, r.Op, right)
}

// compare compares two values: as numbers if both are, else as text regardless of case.
func compare(left any, op string, right any) (bool, error) {
	l, r := text(left), text(right)
	switch op {
	case "empty":
		return empty(left), nil
	case "notEmpty":
		return !empty(left), nil
	case "eq", "ne":
		return equal(left, right) == (op == "eq"), nil
	case "contains", "notContains":
		var has bool
		switch c := left.(type) {
		case []any:
			has = slices.ContainsFunc(c, func(item any) bool { return equal(item, right) })
		case map[string]any:
			_, has = c[r]
		default:
			has = strings.Contains(strings.ToLower(l), strings.ToLower(r))
		}
		return has == (op == "contains"), nil
	case "startsWith":
		return strings.HasPrefix(strings.ToLower(l), strings.ToLower(r)), nil
	case "endsWith":
		return strings.HasSuffix(strings.ToLower(l), strings.ToLower(r)), nil
	case "matches":
		re, err := pattern(r)
		if err != nil {
			return false, err
		}
		return re.MatchString(l), nil
	}
	cmp := strings.Compare(strings.ToLower(l), strings.ToLower(r))
	if a, ok := number(left); ok {
		if b, ok := number(right); ok {
			cmp = 0
			if a < b {
				cmp = -1
			} else if a > b {
				cmp = 1
			}
		}
	}
	switch op {
	case "gt":
		return cmp > 0, nil
	case "ge":
		return cmp >= 0, nil
	case "lt":
		return cmp < 0, nil
	case "le":
		return cmp <= 0, nil
	}
	return false, fmt.Errorf("unknown comparison %q", op)
}

// equal compares values as numbers if both are, else as text regardless of case.
func equal(a, b any) bool {
	if x, ok := number(a); ok {
		if y, ok := number(b); ok {
			return x == y
		}
	}
	return strings.EqualFold(text(a), text(b))
}

// templates calls check for each template in a decoded value, e.g. the settings of a step.
func templates(v any, check func(string) error) error {
	switch v := v.(type) {
	case string:
		if strings.Contains(v, "{{") {
			return check(v)
		}
	case []any:
		for _, item := range v {
			if err := templates(item, check); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range v {
			if err := templates(item, check); err != nil {
				return err
			}
		}
	}
	return nil
}
