// Package rcon runs commands on the consoles of game servers over RCON, the remote console
// protocol of Minecraft (Source RCON).
package rcon

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

// Packet types of the protocol.
const (
	typeResponse = 0
	typeCommand  = 2
	typeAuth     = 3
	maxBytes     = 1 << 16
	// chunk is the size of the packets in which servers split longer answers.
	chunk     = 4096
	maxAnswer = 1 << 20
	// timeout bounds a command whose caller set no deadline, e.g. a save of a big world.
	timeout = 5 * time.Minute
)

var errAuth = errors.New("RCON password rejected")

// conn is a connection to the console of a game server.
type conn struct {
	conn net.Conn
	id   int32
}

func dial(ctx context.Context, addr, password string) (*conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	r := &conn{conn: c}
	if _, err := r.call(ctx, typeAuth, password); err != nil {
		return nil, errors.Join(err, c.Close())
	}
	return r, nil
}

// call sends a packet and returns the body of the answer, until ctx's deadline. An answer
// that fills a packet may go on in the next ones, until the answer to an empty packet sent
// after it, as the server answers in order.
func (r *conn) call(ctx context.Context, typ int32, body string) (string, error) {
	deadline, _ := ctx.Deadline()
	_ = r.conn.SetDeadline(deadline)
	id, err := r.send(typ, body)
	if err != nil {
		return "", err
	}
	got, answer, err := r.receive()
	if err == nil && got != id {
		err = errAuth // the server answers with -1 to a wrong password
	}
	if err != nil || typ != typeCommand || len(answer) < chunk {
		return answer, err
	}
	end, err := r.send(typeResponse, "")
	for err == nil {
		var part string
		got, part, err = r.receive()
		switch {
		case err != nil:
		case got == end:
			return answer, nil
		case got != id || len(answer) > maxAnswer:
			err = errors.New("invalid RCON answer")
		}
		answer += part
	}
	return "", err
}

func (r *conn) send(typ int32, body string) (int32, error) {
	r.id++
	req := binary.LittleEndian.AppendUint32(nil, uint32(10+len(body))) //nolint:gosec // commands are short
	req = binary.LittleEndian.AppendUint32(req, uint32(r.id))          //nolint:gosec // IDs are positive
	req = binary.LittleEndian.AppendUint32(req, uint32(typ))           //nolint:gosec // types are positive
	_, err := r.conn.Write(append(append(req, body...), 0, 0))
	return r.id, err
}

// receive reads a packet and returns the request it answers and its body.
func (r *conn) receive() (int32, string, error) {
	var header [12]byte // length, ID and type
	if _, err := io.ReadFull(r.conn, header[:]); err != nil {
		return 0, "", err
	}
	length := binary.LittleEndian.Uint32(header[:4])
	if length < 10 || length > maxBytes {
		return 0, "", errors.New("invalid RCON packet")
	}
	data := make([]byte, length-8)
	if _, err := io.ReadFull(r.conn, data); err != nil {
		return 0, "", err
	}
	return int32(binary.LittleEndian.Uint32(header[4:8])), string(data[:len(data)-2]), nil //nolint:gosec // two's complement
}

func (r *conn) Close() error { return r.conn.Close() }
