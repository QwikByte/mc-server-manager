package fileset

import (
	"cmp"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/tag"
)

const (
	maxVariables = 50
	maxValues    = 200 // of a variable
	maxValue     = 128 // characters
)

// Variable is a variable of a set, which its files use as {{var:<name>}}, with its values for
// single servers, the servers of networks, those with tags, and all servers. A server gets the
// first value that is for it, in this order, and of those of its tags the first in
// alphabetical order.
type Variable struct {
	Name   string  `json:"name"`
	Values []Value `json:"values"`
}

// Value is the value of a variable for the servers of a kind and scope.
type Value struct {
	Kind  string `json:"kind"`            // server, network, tag or all
	Scope string `json:"scope,omitempty"` // the ID of the server or network, or the tag
	Value string `json:"value"`
}

// precedence orders the kinds of values: a server gets the first that is for it.
var precedence = []string{KindServer, KindNetwork, KindTag, KindAll}

// idPattern matches the IDs of networks and servers, which are random base32 in lowercase.
var idPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)

// checkVariables validates the variables of a set and sorts their values by precedence.
// Values of networks that don't exist are left out.
func checkVariables(variables []Variable, exists func(network string) bool) error {
	if len(variables) > maxVariables {
		return httpapi.Errorf(http.StatusBadRequest, "A file set has up to %d variables.", maxVariables)
	}
	for i := range variables {
		v := &variables[i]
		switch {
		case !noryxv1.SecretName.MatchString(v.Name): // named like secrets
			return httpapi.Errorf(http.StatusBadRequest, "Variable names have up to 64 lower-case letters, digits, - and _.")
		case slices.ContainsFunc(variables[:i], func(o Variable) bool { return o.Name == v.Name }):
			return httpapi.Errorf(http.StatusBadRequest, "The variable %s is there twice.", v.Name)
		case len(v.Values) > maxValues:
			return httpapi.Errorf(http.StatusBadRequest, "A variable has up to %d values.", maxValues)
		}
		for j, val := range v.Values {
			val.Value = strings.TrimSpace(val.Value)
			if problem := valueProblem(val.Value); problem != "" {
				return httpapi.Errorf(http.StatusBadRequest, "%s: %s", v.Name, problem)
			}
			switch val.Kind {
			case KindAll:
				val.Scope = ""
			case KindTag:
				tags, err := tag.Normalize([]string{val.Scope})
				if err != nil {
					return err
				}
				val.Scope = tags[0]
			case KindNetwork, KindServer:
				if !idPattern.MatchString(val.Scope) {
					return httpapi.Errorf(http.StatusBadRequest, "%s: choose a server or network for the value.", v.Name)
				}
			default:
				return httpapi.Errorf(http.StatusBadRequest, "%s: give the value to a server, a network, a tag or all servers.", v.Name)
			}
			v.Values[j] = val
		}
		v.Values = slices.DeleteFunc(v.Values, func(val Value) bool { return val.Kind == KindNetwork && !exists(val.Scope) })
		slices.SortFunc(v.Values, func(a, b Value) int {
			return cmp.Or(cmp.Compare(slices.Index(precedence, a.Kind), slices.Index(precedence, b.Kind)), strings.Compare(a.Scope, b.Scope))
		})
		for j := 1; j < len(v.Values); j++ {
			if v.Values[j].Kind == v.Values[j-1].Kind && v.Values[j].Scope == v.Values[j-1].Scope {
				return httpapi.Errorf(http.StatusBadRequest, "%s has two values for the same servers.", v.Name)
			}
		}
		if v.Values == nil {
			v.Values = []Value{}
		}
	}
	return nil
}

// valueProblem returns why text can't be the value of a variable, or "": like the variables
// of servers, a single line, here without quotes, backslashes and braces, so that it can't add
// lines to a file, end a quoted text or make up a placeholder.
func valueProblem(value string) string {
	if value == "" || utf8.RuneCountInString(value) > maxValue || !utf8.ValidString(value) ||
		strings.ContainsFunc(value, func(r rune) bool { return !unicode.IsPrint(r) || strings.ContainsRune("\"'`\\{}", r) }) {
		return fmt.Sprintf("A value is a single line of up to %d characters without quotes, backslashes and braces.", maxValue)
	}
	return ""
}

// value returns the value of a variable for a server: the first of its values that is for it.
func (v Variable) value(m member) (string, bool) {
	for _, val := range v.Values {
		switch {
		case val.Kind == KindAll, val.Kind == KindServer && val.Scope == m.ServerID,
			val.Kind == KindNetwork && val.Scope == m.NetworkID, val.Kind == KindTag && slices.Contains(m.Tags, val.Scope):
			return val.Value, true
		}
	}
	return "", false
}
