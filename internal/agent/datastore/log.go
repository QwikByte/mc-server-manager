package datastore

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// maxTail is the most past lines of a log that a client gets.
const maxTail = 1000

// passwords matches the passwords in statements that the engines log, e.g. when one fails:
// PASSWORD '…' and PASSWORD('…') of both, and MariaDB's IDENTIFIED BY '…' and IDENTIFIED VIA …
// USING '…', whose hash is as good as the password for some of its plugins. Quotes in them are
// doubled or escaped with a backslash, and a literal that the line cuts off is hidden to its end.
var passwords = regexp.MustCompile(`(?i)((?:PASSWORD\s*\(?|IDENTIFIED\s+BY|USING)\s*)'(?:[^'\\]|''|\\.)*'?`)

// StreamDatastoreLogs streams the log of a datastore's container, cleaned so that it shows no
// passwords and can't control a terminal.
func (s *Service) StreamDatastoreLogs(req *noryxv1.StreamDatastoreLogsRequest, stream noryxv1.DatastoreService_StreamDatastoreLogsServer) error {
	ds, err := s.find(stream.Context(), req.GetId())
	if err != nil {
		return err
	}
	var after time.Time
	if req.GetAfterUnixNano() > 0 {
		after = time.Unix(0, req.GetAfterUnixNano())
	}
	for line, err := range s.rt.DatastoreLogs(stream.Context(), ds.ID, int(min(req.GetTail(), maxTail)), after) {
		if err != nil {
			return toStatus(err)
		}
		res := &noryxv1.StreamDatastoreLogsResponse{Line: clean(line.Text)}
		if !line.Time.IsZero() {
			res.TimeUnixNano = line.Time.UnixNano()
		}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}

// clean drops the control characters of a line, which could control the terminal of the local
// CLI, replaces invalid UTF-8, which strings.Map does on the way, and hides passwords.
func clean(line string) string {
	line = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1
		}
		return r
	}, line)
	return passwords.ReplaceAllString(line, "${1}'<hidden>'")
}
