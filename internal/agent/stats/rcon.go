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
	rconAuth     = 3
	rconCommand  = 2
	maxRCONBytes = 1 << 16
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

// call sends a packet and returns the body of the answer.
func (r *rcon) call(ctx context.Context, typ int32, body string) (string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(probeTimeout)
	}
	_ = r.conn.SetDeadline(deadline)
	r.id++
	req := binary.LittleEndian.AppendUint32(nil, uint32(10+len(body))) //nolint:gosec // commands are short
	req = binary.LittleEndian.AppendUint32(req, uint32(r.id))          //nolint:gosec // IDs are positive
	req = binary.LittleEndian.AppendUint32(req, uint32(typ))           //nolint:gosec // types are positive
	if _, err := r.conn.Write(append(append(req, body...), 0, 0)); err != nil {
		return "", err
	}
	var header [12]byte // length, ID and type
	if _, err := io.ReadFull(r.conn, header[:]); err != nil {
		return "", err
	}
	length := binary.LittleEndian.Uint32(header[:4])
	if length < 10 || length > maxRCONBytes {
		return "", errors.New("invalid RCON packet")
	}
	data := make([]byte, length-8)
	if _, err := io.ReadFull(r.conn, data); err != nil {
		return "", err
	}
	if id := int32(binary.LittleEndian.Uint32(header[4:8])); id != r.id { //nolint:gosec // two's complement
		return "", errRCONAuth // the server answers with -1 to a wrong password
	}
	return string(data[:len(data)-2]), nil
}

func (r *rcon) Close() error { return r.conn.Close() }
