package stats

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

const (
	maxStatusBytes = 1 << 20 // the status may contain the server icon
	maxNames       = 20
	maxNameLen     = 64
)

// ping asks the server at the port of the node for its players with the status request of
// the Minecraft protocol, which the server list in the game sends too. Game servers since
// 1.7 and proxies answer it.
func ping(ctx context.Context, port uint32) (*mcsmv1.Players, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(port), 10)))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	// A handshake to the status state, with an unknown protocol version, and the request.
	handshake := appendString(appendVarint([]byte{0x00}, -1), "127.0.0.1")
	handshake = append(binary.BigEndian.AppendUint16(handshake, uint16(port)), 0x01) //nolint:gosec // ports are 16 bits
	if _, err := conn.Write(append(packet(handshake), packet([]byte{0x00})...)); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	length, err := binary.ReadUvarint(r)
	if err != nil || length > maxStatusBytes {
		return nil, errors.Join(errors.New("invalid status response"), err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	_, n := binary.Uvarint(data) // the packet ID
	size, m := binary.Uvarint(data[max(n, 0):])
	body := data[max(n, 0)+max(m, 0):]
	if n <= 0 || m <= 0 || size != uint64(len(body)) { //nolint:gosec // lengths aren't negative
		return nil, errors.New("invalid status response")
	}
	var status struct {
		Players struct {
			Online uint32 `json:"online"`
			Max    uint32 `json:"max"`
			Sample []struct {
				Name string `json:"name"`
			} `json:"sample"`
		} `json:"players"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, err
	}
	players := &mcsmv1.Players{Online: status.Players.Online, Max: status.Players.Max}
	for _, p := range status.Players.Sample[:min(len(status.Players.Sample), maxNames)] {
		name := plain(p.Name)
		players.Names = append(players.Names, strings.ToValidUTF8(name[:min(len(name), maxNameLen)], ""))
	}
	return players, nil
}

// packet frames the data of a packet with its length.
func packet(data []byte) []byte { return append(appendVarint(nil, int32(len(data))), data...) } //nolint:gosec // packets are small

// appendVarint appends a VarInt of the Minecraft protocol: 7 bits at a time, of the value
// as a 32-bit unsigned integer.
func appendVarint(b []byte, v int32) []byte {
	u := uint32(v) //nolint:gosec // negative values are encoded in two's complement
	for ; u >= 0x80; u >>= 7 {
		b = append(b, byte(u)|0x80)
	}
	return append(b, byte(u))
}

func appendString(b []byte, s string) []byte {
	return append(appendVarint(b, int32(len(s))), s...) //nolint:gosec // strings are short
}
