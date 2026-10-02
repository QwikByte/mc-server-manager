package logging

import (
	"context"
	"log/slog"
	"time"
)

// Entry is a record with its attributes as text. Category, user, node and server are
// taken out of the attributes.
type Entry struct {
	Time       time.Time
	Level      slog.Level
	Message    string
	Category   string
	User       string
	Node       string
	NodeName   string
	Server     string
	ServerName string
	Attrs      map[string]string
}

func (e *Entry) add(prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	switch {
	case a.Key == "" && a.Value.Kind() != slog.KindGroup:
	case a.Value.Kind() == slog.KindGroup:
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, g := range a.Value.Group() {
			e.add(prefix, g)
		}
	case prefix == "" && a.Key == KeyCategory:
		e.Category = a.Value.String()
	case prefix == "" && a.Key == KeyUser:
		e.User = a.Value.String()
	case prefix == "" && a.Key == KeyNode:
		e.Node = a.Value.String()
	case prefix == "" && a.Key == KeyServer:
		e.Server = a.Value.String()
	case prefix == "" && a.Key == KeyNodeName:
		e.NodeName = a.Value.String()
	case prefix == "" && a.Key == KeyServerName:
		e.ServerName = a.Value.String()
	default:
		e.Attrs[prefix+a.Key] = a.Value.String()
	}
}

// NewHandler returns a handler that passes each record of at least the given level to sink
// as an entry, e.g. to store it.
func NewHandler(level slog.Leveler, sink func(Entry)) slog.Handler {
	return &handler{level: level, sink: sink}
}

type handler struct {
	level  slog.Leveler
	sink   func(Entry)
	attrs  []slog.Attr // from WithAttrs, with the group they were added in
	prefix string      // of the current group
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level.Level() }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	e := Entry{Time: r.Time, Level: Normalize(r.Level), Message: r.Message, Category: System.Value.String(), Attrs: map[string]string{}}
	for _, a := range h.attrs {
		e.add("", a)
	}
	r.Attrs(func(a slog.Attr) bool {
		e.add(h.prefix, a)
		return true
	})
	h.sink(e)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append(c.attrs[:len(c.attrs):len(c.attrs)], slog.Attr{Value: slog.GroupValue(attrs...), Key: trimDot(h.prefix)})
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	c := *h
	c.prefix += name + "."
	return &c
}

func trimDot(prefix string) string {
	if prefix == "" {
		return ""
	}
	return prefix[:len(prefix)-1]
}
