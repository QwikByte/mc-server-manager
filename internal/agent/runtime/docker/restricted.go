package docker

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"regexp"
)

// maxHeld is how many bytes of blank lines and \unrestrict a restricted dump holds back to tell
// whether they end it.
const maxHeld = 64 << 10

var (
	restrictLine   = regexp.MustCompile(`^\\restrict [A-Za-z0-9]+\s*$`)
	unrestrictLine = regexp.MustCompile(`^\\unrestrict [A-Za-z0-9]+\s*$`)
)

// restricted returns an SQL dump for psql in its restricted mode, which refuses psql's own
// commands, such as \! that runs programs, \connect, \copy, \i and \o, so that the dump can
// only send statements to the server as the user psql signed in as. It enters the mode with a
// random key, and leaves out the \restrict and \unrestrict with which pg_dump wraps dumps
// since PostgreSQL 17.6, as their author knows the key: the \restrict before the first
// statement and the \unrestrict after the last. Any other one fails the load, also in the
// data of COPY, which pg_dump never writes at the start of a line.
func restricted(r io.Reader) io.Reader {
	return &restrictedDump{in: bufio.NewReader(r), out: []byte(`\restrict ` + rand.Text() + "\n"), header: true}
}

type restrictedDump struct {
	in  *bufio.Reader
	out []byte // to be read
	// header is set before the first statement, while there are only comments and blank lines.
	header bool
	// held are blank lines and \unrestrict that may end the dump.
	held []byte
	// midLine is set while the line that was read last goes on.
	midLine bool
	err     error
}

func (d *restrictedDump) Read(p []byte) (int, error) {
	for len(d.out) == 0 && d.err == nil {
		d.next()
	}
	if len(d.out) == 0 {
		return 0, d.err
	}
	n := copy(p, d.out)
	d.out = d.out[n:]
	return n, nil
}

// next reads a line, or the part of a long one that fits the buffer.
func (d *restrictedDump) next() {
	chunk, err := d.in.ReadSlice('\n')
	whole := !d.midLine
	d.midLine = errors.Is(err, bufio.ErrBufferFull)
	whole = whole && !d.midLine
	line := bytes.TrimRight(chunk, "\r\n")
	blank := len(bytes.TrimSpace(line)) == 0
	switch {
	case whole && d.header && (blank || bytes.HasPrefix(line, []byte("--"))):
		d.out = append(d.out, chunk...)
	case whole && d.header && restrictLine.Match(line):
		d.header = false
	case whole && !d.header && (blank || unrestrictLine.Match(line)) && len(d.held) < maxHeld:
		d.held = append(d.held, chunk...)
	default:
		d.header = false
		d.out = append(append(d.out, d.held...), chunk...)
		d.held = d.held[:0]
	}
	switch {
	case errors.Is(err, io.EOF):
		d.err = io.EOF // what is held only ends the dump
	case err != nil && !d.midLine:
		d.err = err
	}
}
