// Package logging sets up the log of the master and the agent. Records go to the console,
// optionally to a log file that is rotated by size, and to further handlers: the master
// keeps them in its database, the agent in a buffer the master reads.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Attributes with a meaning beyond the record: the master stores them in columns of their
// own, which decide who may see an entry and what it can be filtered by.
const (
	KeyCategory = "category"
	KeyUser     = "user"
	KeyNode     = "node"   // ID of the node an entry is about
	KeyServer   = "server" // ID of the server an entry is about
	// Names of the node and server at the time of the entry, if known.
	KeyNodeName   = "node_name"
	KeyServerName = "server_name"
)

// Categories tell what an entry is about. Records without one belong to System.
var (
	System    = Category("system")
	Auth      = Category("auth")
	Users     = Category("users")
	Settings  = Category("settings")
	Nodes     = Category("nodes")
	Servers   = Category("servers")
	Console   = Category("console")
	Files     = Category("files")
	Plugins   = Category("plugins")
	Backups   = Category("backups")
	Networks  = Category("networks")
	Players   = Category("players")
	Templates = Category("templates")
	Policies  = Category("policies")
	Terminal  = Category("terminal")
	Databases = Category("databases")
	Usage     = Category("usage")
	// Notifications are about the channels that send warnings elsewhere, e.g. to Discord.
	Notifications = Category("notifications")
)

// Category returns the attribute of a category, e.g. for slog.Info("…", logging.Nodes).
func Category(name string) slog.Attr { return slog.String(KeyCategory, name) }

// Options configure the log of a program.
type Options struct {
	Level  slog.Level
	Format string
	File   string
}

// AddFlags adds the flags of the options.
func (o *Options) AddFlags(f *pflag.FlagSet) {
	f.TextVar(&o.Level, "log-level", slog.LevelInfo, "least important level that is logged: debug, info, warn or error")
	f.StringVar(&o.Format, "log-format", "text", "format of the log on stderr: text or json")
	f.StringVar(&o.File, "log-file", "", "also write the log to this file as JSON lines, rotated at 10 MB with 5 older files kept")
}

// Setup makes slog log to stderr, the log file and the given handlers. Close the result
// when the program ends.
func Setup(o Options, handlers ...slog.Handler) (io.Closer, error) {
	opts := &slog.HandlerOptions{Level: o.Level}
	switch o.Format {
	case "text":
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, opts))
	case "json":
		handlers = append(handlers, slog.NewJSONHandler(os.Stderr, opts))
	default:
		return nil, fmt.Errorf("unknown log format %q, use text or json", o.Format)
	}
	var file io.Closer = io.NopCloser(nil)
	if o.File != "" {
		if err := os.MkdirAll(filepath.Dir(o.File), 0o750); err != nil {
			return nil, err
		}
		f, err := openRotating(o.File)
		if err != nil {
			return nil, err
		}
		handlers, file = append(handlers, slog.NewJSONHandler(f, opts)), f
	}
	slog.SetDefault(slog.New(slog.NewMultiHandler(handlers...)))
	return file, nil
}

// Normalize rounds a level down to debug, info, warn or error.
func Normalize(l slog.Level) slog.Level {
	switch {
	case l >= slog.LevelError:
		return slog.LevelError
	case l >= slog.LevelWarn:
		return slog.LevelWarn
	case l >= slog.LevelInfo:
		return slog.LevelInfo
	}
	return slog.LevelDebug
}

// LevelName is the lower-case name of a normalized level, e.g. "warn".
func LevelName(l slog.Level) string { return strings.ToLower(Normalize(l).String()) }

// ParseLevel reads a level name such as "warn" or "error".
func ParseLevel(name string) (slog.Level, error) {
	var l slog.Level
	err := l.UnmarshalText([]byte(name))
	if err != nil {
		err = errors.New("choose the level debug, info, warn or error")
	}
	return Normalize(l), err
}

// Line formats an entry for terminals: time, level, category, message and the attributes
// sorted by key.
func Line(t time.Time, level slog.Level, category, message string, attrs map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %-5s  %-9s  %s", t.Local().Format(time.DateTime), strings.ToUpper(LevelName(level)), category, message)
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		fmt.Fprintf(&b, "  %s=%q", k, attrs[k])
	}
	return b.String()
}
