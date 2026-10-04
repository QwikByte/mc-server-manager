package stats

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

// RCON packet types (Source RCON protocol, which Minecraft implements).
const (
	rconResponse = 0
	rconAuth     = 3
	rconCommand  = 2
	maxRCONBytes = 1 << 16
	// rconChunk is the size of the packets in which servers split longer answers.
	rconChunk     = 4096
	maxRCONAnswer = 1 << 20
)

var errRCONAuth = errors.New("RCON password rejected")

// rcon is a connection to the console of a game server. It stays open, because the server
// logs every new one.
type rcon struct {
	conn net.Conn
	id   int32
}

func dialRCON(ctx context.Context, addr, password string) (*rcon, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	r := &rcon{conn: conn}
	if _, err := r.call(ctx, rconAuth, password); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return r, nil
}

// call sends a packet and returns the body of the answer. An answer that fills a packet
// may go on in the next ones, until the answer to an empty packet sent after it, as the
// server answers in order.
func (r *rcon) call(ctx context.Context, typ int32, body string) (string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(probeTimeout)
	}
	_ = r.conn.SetDeadline(deadline)
	id, err := r.send(typ, body)
	if err != nil {
		return "", err
	}
	got, answer, err := r.receive()
	if err == nil && got != id {
		err = errRCONAuth // the server answers with -1 to a wrong password
	}
	if err != nil || typ != rconCommand || len(answer) < rconChunk {
		return answer, err
	}
	end, err := r.send(rconResponse, "")
	for err == nil {
		var part string
		got, part, err = r.receive()
		switch {
		case err != nil:
		case got == end:
			return answer, nil
		case got != id || len(answer) > maxRCONAnswer:
			err = errors.New("invalid RCON answer")
		}
		answer += part
	}
	return "", err
}

func (r *rcon) send(typ int32, body string) (int32, error) {
	r.id++
	req := binary.LittleEndian.AppendUint32(nil, uint32(10+len(body))) //nolint:gosec // commands are short
	req = binary.LittleEndian.AppendUint32(req, uint32(r.id))          //nolint:gosec // IDs are positive
	req = binary.LittleEndian.AppendUint32(req, uint32(typ))           //nolint:gosec // types are positive
	_, err := r.conn.Write(append(append(req, body...), 0, 0))
	return r.id, err
}

// receive reads a packet and returns the request it answers and its body.
func (r *rcon) receive() (int32, string, error) {
	var header [12]byte // length, ID and type
	if _, err := io.ReadFull(r.conn, header[:]); err != nil {
		return 0, "", err
	}
	length := binary.LittleEndian.Uint32(header[:4])
	if length < 10 || length > maxRCONBytes {
		return 0, "", errors.New("invalid RCON packet")
	}
	data := make([]byte, length-8)
	if _, err := io.ReadFull(r.conn, data); err != nil {
		return 0, "", err
	}
	return int32(binary.LittleEndian.Uint32(header[4:8])), string(data[:len(data)-2]), nil //nolint:gosec // two's complement
}

func (r *rcon) Close() error { return r.conn.Close() }
