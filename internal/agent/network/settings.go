package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxChanges = 200
	maxValue   = 4096
	maxItems   = 256
)

// segmentPattern matches the keys a setting's path is made of; keys with other characters,
// such as the dots of host names, can't be told apart from the path.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Settings returns the settings of a proxy's configuration by their path, e.g.
// "advanced.compression-threshold" or "listeners.0.motd", as JSON: strings, numbers,
// booleans and lists of strings. The settings that the manager or the network decides are
// left out and returned in locked, with the reason.
func (p Proxy) Settings(data []byte) (settings, locked map[string]string, err error) {
	doc, err := parse(p.File, data)
	if err != nil {
		return nil, nil, err
	}
	values, locked := p.values(doc)
	settings = make(map[string]string, len(values))
	for path, v := range values {
		var j strings.Builder
		e := json.NewEncoder(&j)
		e.SetEscapeHTML(false) // MiniMessage tags are no HTML
		if e.Encode(v) == nil {
			settings[path] = strings.TrimSpace(j.String())
		}
	}
	return settings, locked, nil
}

// Change sets settings of a proxy's configuration, given as JSON by their path like
// Settings returns them, and keeps the others. Each must be in the file already, can't be
// locked and keeps its kind. The error tells the operator what is wrong.
func (p Proxy) Change(data []byte, changes map[string]string) ([]byte, error) {
	if len(changes) > maxChanges {
		return nil, errors.New("too many changes at once")
	}
	doc, err := parse(p.File, data)
	if err != nil {
		return nil, err
	}
	values, _ := p.values(doc)
	for path, raw := range changes {
		current, ok := values[path]
		switch {
		case p.lockedAt(path) != "":
			return nil, fmt.Errorf("%s can't be changed: %s", path, p.lockedAt(path))
		case !ok:
			return nil, fmt.Errorf("%s has no setting %s", p.File, path)
		}
		value, err := convert(current, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		set(doc, strings.Split(path, "."), value)
	}
	return formatOf(p.File).marshal(doc)
}

// values returns the settings of a configuration by path, and the locked ones with the reason.
func (p Proxy) values(doc map[string]any) (values map[string]any, locked map[string]string) {
	values, locked = map[string]any{}, map[string]string{}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		if reason := p.lockedAt(path); reason != "" {
			locked[path] = reason
			return
		}
		switch v := v.(type) {
		case map[string]any:
			for key, child := range v {
				if segmentPattern.MatchString(key) {
					walk(strings.TrimPrefix(path+"."+key, "."), child)
				}
			}
		case []any:
			if list, ok := texts(v); ok {
				values[path] = list
				return
			}
			for i, child := range v {
				if _, ok := child.(map[string]any); ok {
					walk(path+"."+strconv.Itoa(i), child)
				}
			}
		case string, bool, int, int64, uint64, float64:
			values[path] = v
		}
	}
	walk("", doc)
	return values, locked
}

// lockedAt returns why the setting at path, or a setting it is part of, is locked, or ""
// if it isn't.
func (p Proxy) lockedAt(path string) string {
	segments := strings.Split(path, ".")
	for i, s := range segments {
		if _, err := strconv.Atoi(s); err == nil {
			segments[i] = "*"
		}
		if reason := p.managed[strings.Join(segments[:i+1], ".")]; reason != "" {
			return reason
		}
	}
	return ""
}

// convert decodes a new value from JSON, which must be of the same kind as the current one.
func convert(current any, raw string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, errors.New("invalid value")
	}
	switch current.(type) {
	case bool:
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return nil, errors.New("choose on or off")
	case int, int64, uint64:
		f, ok := v.(float64)
		if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
			return nil, errors.New("enter a whole number")
		}
		if _, ok := current.(int); ok {
			return int(f), nil
		}
		return int64(f), nil
	case float64:
		if f, ok := v.(float64); ok {
			return f, nil
		}
		return nil, errors.New("enter a number")
	case string:
		s, ok := v.(string)
		if !ok || !plain(s, maxValue) {
			return nil, fmt.Errorf("enter text of up to %d characters without control characters", maxValue)
		}
		return s, nil
	case []string:
		list, _ := v.([]any)
		texts, ok := texts(list)
		if !ok || list == nil || len(texts) > maxItems || slices.ContainsFunc(texts, func(s string) bool { return !plain(s, 256) }) {
			return nil, fmt.Errorf("enter up to %d entries of up to 256 characters", maxItems)
		}
		return texts, nil
	}
	return nil, errors.New("can't be changed here")
}

// set puts a value at the path, which exists.
func set(node any, path []string, value any) {
	key := path[0]
	switch n := node.(type) {
	case map[string]any:
		if len(path) == 1 {
			n[key] = value
			return
		}
		set(n[key], path[1:], value)
	case []any:
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(n) {
			set(n[i], path[1:], value)
		}
	}
}

// texts returns the strings of a list that only holds strings; an empty list is one too.
func texts(list []any) ([]string, bool) {
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func plain(s string, limit int) bool {
	return len(s) <= limit && !strings.ContainsFunc(s, func(r rune) bool { return r != '\n' && unicode.IsControl(r) })
}
