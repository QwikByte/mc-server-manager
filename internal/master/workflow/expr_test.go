package workflow

import (
	"reflect"
	"testing"
)

func TestTemplates(t *testing.T) {
	data := map[string]any{
		"trigger": map[string]any{"server": map[string]any{"name": "Lobby"}, "players": []any{"Alex", "Steve"}},
		"steps":   map[string]any{"find": map[string]any{"count": 2.0, "servers": []any{map[string]any{"name": "A"}, map[string]any{"name": "B"}}}},
		"text":    " Hello ",
	}
	get := func(path []string) any { return lookupIn(data, path) }
	for tmpl, want := range map[string]any{
		"plain":                                      "plain",
		"{{trigger.server.name}}":                    "Lobby",
		"Hi {{trigger.server.name}}!":                "Hi Lobby!",
		"{{trigger.players}}":                        []any{"Alex", "Steve"},
		"{{trigger.players | join}}":                 "Alex, Steve",
		"{{trigger.players | join:' & '}}":           "Alex & Steve",
		"{{trigger.players | length}}":               2.0,
		"{{trigger.players.1}}":                      "Steve",
		"{{trigger.players | last | upper}}":         "STEVE",
		"{{steps.find.count | add:40}}":              42.0,
		"{{steps.find.count | div:3 | round:2}}":     0.67,
		"{{steps.find.servers | map:name | join:,}}": "A,B",
		"{{missing | default:none}}":                 "none",
		"{{text | trim | lower}}":                    "hello",
		"{{'a,b, c' | split}}":                       nil, // a quoted path is no path
		"{{trigger.server | json}}":                  `{"name":"Lobby"}`,
		"{{trigger.server.name | replace:Lob:Hub}}":  "Hubby",
		"n={{steps.find.count}}":                     "n=2",
	} {
		parsed, err := parse(tmpl)
		if want == nil {
			if err == nil {
				t.Errorf("%s: parsed", tmpl)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tmpl, err)
			continue
		}
		if got, err := parsed.eval(get); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, %v; want %#v", tmpl, got, err, want)
		}
	}
	for _, bad := range []string{"{{", "{{ }}", "{{a | nofilter}}", "{{a | join:x:y}}", "{{a | default:'x}}"} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if _, err := mustParse(t, "{{text | add:1}}").eval(get); err == nil {
		t.Error("added to text")
	}
}

func lookupIn(data map[string]any, path []string) any { return get(data, path) }

func mustParse(t *testing.T, s string) tmpl {
	t.Helper()
	p, err := parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConditions(t *testing.T) {
	values := map[string]any{"10": "10", "9": 9.0, "lobby": "Lobby-1", "list": []any{"a", "b"}, "": "", "time": "08:30"}
	value := func(s string) (any, error) { return values[s], nil }
	for _, c := range []struct {
		left, op, right string
		want            bool
	}{
		{"10", "gt", "9", true}, // as numbers, not as text
		{"9", "lt", "10", true},
		{"10", "eq", "10", true},
		{"lobby", "startsWith", "lobby", true}, // regardless of case
		{"lobby", "contains", "BY-", true},
		{"lobby", "matches", "^lobby", false}, // regular expressions keep their case
		{"list", "contains", "b", true},
		{"list", "notContains", "c", true},
		{"", "empty", "", true},
		{"list", "notEmpty", "", true},
		{"time", "lt", "10", true}, // "08:30" < "10" as text
	} {
		right := c.right
		if _, ok := values[right]; !ok {
			values[right] = right
		}
		cond := Condition{Match: "all", Rules: []Rule{{Left: c.left, Op: c.op, Right: right}}}
		if got, err := cond.eval(value); err != nil || got != c.want {
			t.Errorf("%s %s %s = %v, %v", c.left, c.op, c.right, got, err)
		}
	}
	either := Condition{Match: "any", Rules: []Rule{
		{Left: "9", Op: "gt", Right: "10"},
		{Group: &Condition{Match: "all", Rules: []Rule{{Left: "list", Op: "contains", Right: "b"}, {Left: "", Op: "empty"}}}},
	}}
	if ok, err := either.eval(value); !ok || err != nil {
		t.Errorf("either = %v, %v", ok, err)
	}
	for _, bad := range []Condition{
		{Match: "some", Rules: []Rule{{Op: "eq"}}},
		{Match: "all"},
		{Match: "all", Rules: []Rule{{Op: "like"}}},
		{Match: "all", Rules: []Rule{{Op: "matches", Right: "("}}},
		{Match: "all", Rules: []Rule{{Group: &Condition{Match: "all", Rules: []Rule{{Group: &Condition{Match: "all", Rules: []Rule{{Group: &Condition{Match: "all", Rules: []Rule{{Op: "eq"}}}}}}}}}}}},
	} {
		if bad.check(1) == nil {
			t.Errorf("%+v passed", bad)
		}
	}
}
