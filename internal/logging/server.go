package logging

import (
	"context"
	"log"
	"log/slog"
	"strings"
)

// ServerErrors returns the ErrorLog of an http.Server, which logs to l. TLS handshake
// errors, which every browser causes that doesn't trust the panel's certificate, are only
// logged at debug, recovered panics at error and everything else at warn.
func ServerErrors(l *slog.Logger) *log.Logger {
	return log.New(serverErrors{l}, "", 0)
}

type serverErrors struct{ l *slog.Logger }

func (s serverErrors) Write(p []byte) (int, error) {
	msg, stack, _ := strings.Cut(strings.TrimSpace(string(p)), "\n")
	level := slog.LevelWarn
	switch {
	case strings.Contains(msg, "TLS handshake error"):
		level = slog.LevelDebug
	case strings.Contains(msg, ": panic serving "):
		level = slog.LevelError
	}
	var attrs []slog.Attr
	if stack != "" {
		attrs = append(attrs, slog.String("stack", stack))
	}
	s.l.LogAttrs(context.Background(), level, msg, attrs...)
	return len(p), nil
}
